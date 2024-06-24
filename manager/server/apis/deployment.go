package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"manager/config"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"

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
					Name:            "cloudgame-container",
					Image:           "cloudgame:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("25m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.FromInt(8080),
							},
						},
						InitialDelaySeconds: 15,
						PeriodSeconds:       10,
						TimeoutSeconds:      5,
						SuccessThreshold:    1,
						FailureThreshold:    3,
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
		log.Printf("Failed to get request data: %v", err)
		return
	}

	// 获取中心站点可用弹性实例之前刷新一遍实例状态
	log.Println("Check started")
	if err := ensureK8sDBConsistency(reqBody.ZoneId); err != nil {
		log.Printf("Failed to synchronize instance status: %v", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}
	log.Println("Check ended")

	availableInstances, err := mysql_service.GetAvailableInstanceInCenter(reqBody.ZoneId)
	if err != nil {
		log.Printf("Failed to get available instances and delete them: %v", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	replica := *reqBody.Missing - availableInstances

	log.Println("Manage started")
	defer log.Println("Manage ended")
	if replica == 0 {
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "Replica is 0, there is no need to apply or release instances",
		}, http.StatusOK)
		log.Printf("%s: Replica is 0, there is no need to apply or release instances", reqBody.ZoneId)
	} else if replica > 0 {
		if err := apply(reqBody.ZoneId, replica); err != nil {
			log.Printf("Failed to apply instances: %v", err)
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
		log.Printf("%s: %d instances applied successfully", reqBody.ZoneId, replica)

	} else {
		if err := release(reqBody.ZoneId, -replica); err != nil {
			log.Printf("Failed to release instances: %v", err)
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
		log.Printf("%s: %d instances released successfully", reqBody.ZoneId, -replica)
	}
}

// 判断Pod是否处于Ready状态
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning { // 必须先处于Runing状态才能判断是否Ready
		return false
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// 过滤得到状态为Ready的实例
func filterReadyPods(podList *corev1.PodList) []*corev1.Pod {
	readyPods := make([]*corev1.Pod, 0)
	for _, pod := range podList.Items {
		if isPodReady(&pod) {
			readyPods = append(readyPods, &pod)
		}
	}
	return readyPods
}

func ensureK8sDBConsistency(zoneId string) error {
	// 1. 从集群中获取对应zone下的实例
	podLabelSelector := fmt.Sprintf("zone_id=%s", zoneId)
	podList, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).List(context.Background(), metav1.ListOptions{
		LabelSelector: podLabelSelector,
	})
	readyPods := filterReadyPods(podList)

	if err != nil {
		return fmt.Errorf("failed to get pod list from kubernetes when checking: %w", err)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
		ch    = make(chan struct{}, 50)
	)

	// 2. 遍历pod，确认和数据库状态一致
	for _, pod := range readyPods {
		wg.Add(1)
		ch <- struct{}{}
		go func(pod *corev1.Pod) {
			defer wg.Done()
			defer func() { <-ch }()

			instanceName := fmt.Sprintf("instance-%s", pod.Name)
			serviceName := fmt.Sprintf("service-%s", pod.Name)
			nodePort := int32(0)

			// 从service获取port
			service, err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Get(context.Background(), serviceName, metav1.GetOptions{})
			if err != nil {
				log.Printf("Failed to get service when checking: %v", err)
				return
			}
			for _, port := range service.Spec.Ports {
				if port.NodePort != 0 {
					nodePort = port.NodePort
					break
				}
			}
			if nodePort == 0 {
				log.Printf("Failed to get port of %s when checking", service)
				return
			}

			if err := checkInstanceStatus(zoneId, pod.Status.HostIP, nodePort, instanceName); err != nil {
				log.Printf("Failed to check instance status: %v", err)
				return
			}

			mu.Lock()
			count++
			mu.Unlock()
		}(pod)
	}
	wg.Wait()

	if int(count) != len(readyPods) {
		log.Printf("There are %d ready instances in %s totally, and %d instances failed to synchronized", len(readyPods), zoneId, len(readyPods)-int(count))
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
		return fmt.Errorf("failed to synchronize instance status for %s: %w", instanceName, err)
	}

	return nil
}

func apply(zoneId string, replica int32) error {
	log.Printf("Trying to deploy %d pods in %s", replica, zoneId)
	podList, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("zone_id=%s, is_elastic=1", zoneId),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods :%v", err)
	}
	if len(podList.Items) >= config.CENTERMAXTOTAL {
		return fmt.Errorf("the number of elastic instance in center is already full")
	}
	leftNumber := int32(config.CENTERMAXTOTAL) - int32(len(podList.Items))
	if leftNumber < replica {
		replica = leftNumber
		log.Printf("But the left space can only deploy %d pods in %s", replica, zoneId)
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
		ch    = make(chan struct{}, 50)
	)
	for i := 1; i <= int(replica); i++ {
		wg.Add(1)
		ch <- struct{}{}
		go func(zoneId string) {
			defer wg.Done()
			defer func() { <-ch }()

			randomId := uuid.NewUUID()
			podName := fmt.Sprintf("cloudgame-center-%s", randomId)
			serviceName := fmt.Sprintf("service-%s", podName)
			instanceId := fmt.Sprintf("instance-%s", podName)

			// 部署Pod和Service
			pod := podFactory(instanceId, podName, zoneId)
			if _, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
				log.Printf("Failed to create pod when applying: %v", err)
				return
			}
			service := serviceFactory(serviceName, instanceId)
			if _, err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
				log.Printf("Failed to create service when applying: %v", err)
				return
			}

			// 获取服务端口
			var nodePort int32
			for nodePort == 0 {
				service, err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Get(context.Background(), serviceName, metav1.GetOptions{})
				if err != nil {
					log.Printf("Failed to get service when applying: %v", err)
					return
				}
				for _, port := range service.Spec.Ports {
					if port.NodePort != 0 {
						nodePort = port.NodePort
						break
					}
				}
			}

			// 循环等待直到Pod部署完成
			var (
				pollInterval    = 2 * time.Second // 轮询间隔
				timeoutDuration = 4 * time.Minute // 超时时长
			)

			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
			defer cancel()

			err := wait.PollUntilContextTimeout(ctx, pollInterval, timeoutDuration, true, func(ctx context.Context) (bool, error) {
				var err error
				pod, err = k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Get(ctx, podName, metav1.GetOptions{})
				if err != nil {
					return false, nil // 如果有错误发生，直接返回
				}
				return isPodReady(pod), nil // 如果 Pod 就绪，返回 true
			})

			if err != nil {
				log.Printf("Timed out waiting for %s in %s to be ready, starting cleanup...", podName, zoneId)
				// 超时的话删除该Pod和Service避免影响K8S集群
				if err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Delete(context.Background(), podName, metav1.DeleteOptions{
					GracePeriodSeconds: func() *int64 { t := int64(0); return &t }(),
				}); err != nil {
					log.Printf("Failed to delete pod %s in namesapce %s when applying: %v", podName, config.K8SNAMSPACE, err)
				}
				if err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Delete(context.Background(), serviceName, metav1.DeleteOptions{}); err != nil {
					log.Printf("Failed to delete service %s in namesapce %s when applying: %v", serviceName, config.K8SNAMSPACE, err)
				}
				return
			}

			// 部署完成后才能保存实例到数据库
			err = mysql_service.InsertInstance(zoneId, "null", pod.Status.HostIP, instanceId, podName, int(nodePort), 1, "available", "null")
			if err != nil {
				log.Printf("Failed to insert instance into database  when applying: %v", err)
				return
			}

			// 计算成功部署的实例数量
			mu.Lock()
			count++
			mu.Unlock()
		}(zoneId)
	}

	wg.Wait()

	if count != replica {
		return fmt.Errorf("%d instances need to be applied, but %d instances failed to apply", replica, replica-count)
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

			var ok = true
			if err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Delete(context.Background(), podName, metav1.DeleteOptions{
				GracePeriodSeconds: func() *int64 { t := int64(0); return &t }(),
			}); err != nil {
				log.Printf("Failed to delete pod %s in namesapce %s when releasing: %v", podName, config.K8SNAMSPACE, err)
				ok = false
			}

			serviceName := fmt.Sprintf("service-%s", podName)
			if err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Delete(context.Background(), serviceName, metav1.DeleteOptions{}); err != nil {
				log.Printf("Failed to delete service %s in namesapce %s when releasing: %v", serviceName, config.K8SNAMSPACE, err)
				ok = false
			}

			if ok {
				mu.Lock()
				count++
				mu.Unlock()
			}

		}(podName)
	}

	wg.Wait()

	if count != replica {
		return fmt.Errorf("%d instances need to be released, but %d instances failed to release", replica, replica-count)
	}

	return nil
}
