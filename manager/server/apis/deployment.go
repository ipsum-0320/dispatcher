package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"manager/config"
	k8s_client "manager/k8s-client"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"

	mysql_service "manager/mysql/service"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func podFactory(
	podName string,
	zoneId string,
) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: config.K8SNAMSPACE,
			Labels: map[string]string{
				"instance": podName,
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
							corev1.ResourceCPU:    resource.MustParse("20m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
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
	podName string,
) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: config.K8SNAMSPACE,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"instance": podName,
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

type InstanceRequest struct {
	ZoneId string `json:"zone_id"`
	Number int32  `json:"number"`
}

func getRequestData(w http.ResponseWriter, r *http.Request) (bool, InstanceRequest) {
	reqBody := InstanceRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		fmt.Printf("Failed to decode request: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, err.Error())
		return false, reqBody
	}

	if reqBody.Number == 0 || reqBody.ZoneId == "" {
		fmt.Println("Number or zone_id is not specificed")
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, "Number or zone_id is not specificed")
		return false, reqBody
	}
	return true, reqBody
}

func InstanceApply(w http.ResponseWriter, r *http.Request) {

	ok, reqBody := getRequestData(w, r)
	if !ok {
		return
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = 0
		err   error
	)
	for i := 1; i <= int(reqBody.Number); i++ {

		wg.Add(1)
		go func(zoneId string) {
			defer wg.Done()

			randomId := uuid.NewUUID()
			podName := fmt.Sprintf("cloudgame-center-%s", randomId)
			serviceName := fmt.Sprintf("service-%s", podName)
			instanceId := fmt.Sprintf("instance-%s", podName)

			// 部署Pod和Service
			pod := podFactory(podName, zoneId)
			if _, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
				fmt.Printf("Failed to create pod: %v\n", err)
				return
			}
			service := serviceFactory(serviceName, podName)
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

		}(reqBody.ZoneId)
	}

	wg.Wait()

	if count == int(reqBody.Number) {
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "All instances applied successfully",
		}, http.StatusOK)
		fmt.Printf("%d instances applied successfully", count)
	} else {
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, "There was something wrong when applying some instances")
		fmt.Printf("There is something wrong: %d instances failed to apply", int(reqBody.Number)-count)
	}
}

func InstanceRelease(w http.ResponseWriter, r *http.Request) {
	ok, reqBody := getRequestData(w, r)
	if !ok {
		return
	}

	podList, err := mysql_service.GetAndDeleteAvailableInstancesInCenter(reqBody.ZoneId, reqBody.Number)
	if err != nil {
		fmt.Printf("Failed to get and delete instances in %s center\n", reqBody.ZoneId)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, "There was something wrong when get available instances from database")
		return
	} else if podList == nil {
		fmt.Printf("There is no elastic instance in %s center\n", reqBody.ZoneId)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, "There is no elastic instance in this zone")
		return
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = 0
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

	if count == int(reqBody.Number) {
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "All instances release successfully",
		}, http.StatusOK)
		fmt.Printf("%d instances released successfully", count)
	} else {
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, "There was something wrong when releasing some instances")
		fmt.Printf("There is something wrong: %d instances failed to release", int(reqBody.Number)-count)
	}
}
