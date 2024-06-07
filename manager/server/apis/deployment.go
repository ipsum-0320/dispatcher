package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"manager/config"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"

	mysql_service "manager/mysql/service"

	k8s_client "manager/k8s-client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func podFactory(
	instanceId string,
	podName string,
	zoneId string,
) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: config.K8SNAMSPACE,
			Labels: map[string]string{
				"instance_id": instanceId,
				"zone_id":     zoneId,
				"is_elastic":  "1",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "cloudgame-container",
					Image: "cloudgame:v1",
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "zone_id",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zoneId},
									},
									{
										Key:      "role",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"center"},
									},
								},
							},
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

func serviceFactory(
	serviceName string,
	instanceId string,
) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: config.K8SNAMSPACE,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"instance_id": instanceId,
			},
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 8080},
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

type InstanceManageRequest struct {
	ZoneId  string `json:"zone_id"`
	Missing *int32 `json:"missing"`
}

func getRequestData(w http.ResponseWriter, r *http.Request) (InstanceManageRequest, error) {
	reqBody := InstanceManageRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {

		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, err.Error())
		return reqBody, fmt.Errorf("failed to decode request: %v", err)
	}

	if reqBody.Missing == nil || reqBody.ZoneId == "" {
		fmt.Println("Number or zone_id is not specificed")
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, "Number or zone_id is not specificed")
		return reqBody, fmt.Errorf("zone_id or missing is not specificed")
	}
	return reqBody, nil
}

func InstanceManage(w http.ResponseWriter, r *http.Request) {
	reqBody, err := getRequestData(w, r)
	if err != nil {
		fmt.Printf("Failed to get request data: %v\n", err)
		return
	}

	// 获取中心站点可用弹性实例之前刷新一遍实例状态
	if err := ensureK8sDBConsistency(reqBody.ZoneId); err != nil {
		fmt.Printf("Failed to synchronize instance status: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	availableInstances, err := mysql_service.GetAvailableInstanceInCenter(reqBody.ZoneId)
	if err != nil {
		fmt.Printf("Failed to get available instances and delete them: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	replica := *reqBody.Missing - availableInstances
	if replica == 0 {
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "Replica is 0, there is no need to apply or release instances",
		}, http.StatusOK)
		fmt.Println("Replica is 0, there is no need to apply or release instances")
	} else if replica > 0 {
		if err := apply(reqBody.ZoneId, replica); err != nil {
			fmt.Printf("Failed to apply instances: %v\n", err)
			SendErrorResponse(w, &ErrorCodeWithMessage{
				HttpStatus: http.StatusInternalServerError,
				ErrorCode:  500,
				Message:    "Internal server error",
			}, err.Error())
			return
		}
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "All instances applied successfully",
		}, http.StatusOK)
		fmt.Printf("%d instances applied successfully\n", replica)
	} else {
		if err := release(reqBody.ZoneId, -replica); err != nil {
			fmt.Printf("Failed to release instances: %v\n", err)
			SendErrorResponse(w, &ErrorCodeWithMessage{
				HttpStatus: http.StatusInternalServerError,
				ErrorCode:  500,
				Message:    "Internal server error",
			}, err.Error())
			return
		}
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "All instances release successfully",
		}, http.StatusOK)
		fmt.Printf("%d instances released successfully\n", -replica)
	}
}

func ensureK8sDBConsistency(zoneId string) error {
	// 1. 从集群中获取对应zone下的弹性实例
	podLabelSelector := fmt.Sprintf("zone_id=%s,is_elastic=1", zoneId)
	podList, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).List(context.Background(), metav1.ListOptions{
		LabelSelector: podLabelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to get pod list from kubernetes: %w", err)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
	)

	// 2. 遍历pod，确认和数据库状态一致
	for _, pod := range podList.Items {
		wg.Add(1)
		go func(pod corev1.Pod) {
			defer wg.Done()
			instanceName := fmt.Sprintf("instance-%s", pod.Name)
			serviceName := fmt.Sprintf("service-%s", pod.Name)
			nodePort := int32(0)

			// 从service获取port
			service, err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Get(context.Background(), serviceName, metav1.GetOptions{})
			if err != nil {
				fmt.Printf("Failed to get service: %v\n", err)
				return
			}
			for _, port := range service.Spec.Ports {
				if port.NodePort != 0 {
					nodePort = port.NodePort
					break
				}
			}
			if nodePort == 0 {
				fmt.Printf("Failed to get port of %s\n", service)
				return
			}

			if err := checkInstanceStatus(zoneId, pod.Status.PodIP, nodePort, instanceName); err != nil {
				fmt.Printf("Failed to check instance status: %v\n", err)
				return
			}

			mu.Lock()
			count++
			mu.Unlock()
		}(pod)
	}
	wg.Wait()

	if int(count) != len(podList.Items) {
		fmt.Printf("There are %d elastic instances in %s totally, and %d instances failed to synchronized\n", len(podList.Items), zoneId, len(podList.Items)-int(count))
	}
	return nil
}

func checkInstanceStatus(zoneId string, host string, port int32, instanceName string) error {
	url := fmt.Sprintf("http://%s:%d/getStatus", host, port)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error fetching status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	status := string(body)

	if err := mysql_service.SynchronizeInstanceStatus(zoneId, instanceName, status); err != nil {
		fmt.Printf("Failed to synchronize instance status for %s: %v\n", instanceName, err)
	}

	return nil
}

func apply(zoneId string, replica int32) error {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
		err   error
	)
	for i := 1; i <= int(replica); i++ {

		wg.Add(1)
		go func(zoneId string) {
			defer wg.Done()

			randomId := uuid.NewUUID()
			podName := fmt.Sprintf("cloudgame-center-%s", randomId)
			serviceName := fmt.Sprintf("service-%s", podName)
			instanceId := fmt.Sprintf("instance-%s", podName)

			// 部署Pod和Service
			pod := podFactory(instanceId, podName, zoneId)
			if _, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
				fmt.Printf("Failed to create pod: %v\n", err)
				return
			}
			service := serviceFactory(serviceName, instanceId)
			if _, err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Create(context.TODO(), service, metav1.CreateOptions{}); err != nil {
				fmt.Printf("Failed to create service: %v\n", err)
				return
			}

			// 等待Pod和Service部署完成
			time.Sleep(1 * time.Second)
			if pod, err = k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Get(context.Background(), podName, metav1.GetOptions{}); err != nil {
				fmt.Printf("Failed to get pod: %v\n", err)
				return
			}
			if service, err = k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Get(context.Background(), serviceName, metav1.GetOptions{}); err != nil {
				fmt.Printf("Failed to get service: %v\n", err)
				return
			}

			for _, port := range service.Spec.Ports {
				if port.NodePort != 0 {
					err := mysql_service.InsertInstance(zoneId, "null", pod.Status.HostIP, instanceId, podName, int(port.NodePort), 1, "available", "null")
					if err != nil {
						fmt.Printf("Failed to insert instance into database: %v\n", err)
						return
					}
					mu.Lock()
					count++
					mu.Unlock()
					break
				}
			}
		}(zoneId)
	}

	wg.Wait()

	if count != replica {
		return fmt.Errorf("%d instances failed to apply", replica-count)
	}

	return nil
}

func release(zoneId string, replica int32) error {

	podList, err := mysql_service.GetAndDeleteAvailableInstancesInCenter(zoneId, replica)
	if err != nil {
		return fmt.Errorf("failed to get available instances from database")
	} else if podList == nil {
		return fmt.Errorf("there is no elastic instance in this zone")
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
	)
	for _, podName := range podList {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()

			if err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Delete(context.TODO(), podName, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					fmt.Printf("Pod %s not found in namespace %s\n", podName, config.K8SNAMSPACE)
				} else {
					fmt.Printf("Failed to delete pod %s in namesapce %s: %v\n", podName, config.K8SNAMSPACE, err)
				}
				return
			}

			serviceName := fmt.Sprintf("service-%s", podName)
			if err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Delete(context.TODO(), serviceName, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					fmt.Printf("Service %s not found in namespace %s\n", serviceName, config.K8SNAMSPACE)
				} else {
					fmt.Printf("Failed to delete service %s in namesapce %s: %v\n", serviceName, config.K8SNAMSPACE, err)
				}
				return
			}
			mu.Lock()
			count++
			mu.Unlock()
		}(podName)
	}

	wg.Wait()

	if count != replica {
		return fmt.Errorf("%d instances failed to release", replica-count)
	}

	return nil
}
