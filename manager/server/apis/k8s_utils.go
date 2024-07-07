package apis

import (
	"context"
	"fmt"
	"log"
	"manager/config"
	mysql_service "manager/mysql/service"
	"strings"
	"sync"
	"time"

	k8s_client "manager/k8s-client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
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

func createAndWatchPod(podName string, instanceId string, zoneId string) (string, int32, error) {
	pod := podFactory(instanceId, podName, zoneId)
	_, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("error creating pod: %w", err)
	}

	serviceName := fmt.Sprintf("service-%s", podName)
	service := serviceFactory(serviceName, instanceId)
	if service, err = k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
		return "", 0, fmt.Errorf("error creating service: %w", err)
	}
	nodePort := service.Spec.Ports[0].NodePort

	// 监控Pod状态，设置超时时间
	timeoutDuration := 3 * time.Minute // 超时时长
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	watchInterface, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
	})
	if err != nil {
		return "", 0, fmt.Errorf("error watching Pod: %w", err)
	}

	ch := watchInterface.ResultChan()
	for {
		select {
		case event := <-ch:
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				return "", 0, fmt.Errorf("error watching: unexpected type")
			}

			if event.Type == watch.Modified {
				if pod.Status.Phase == corev1.PodRunning {
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							serverIp := pod.Status.HostIP
							if serverIp != "" && checkPodReady(serverIp, nodePort) {
								watchInterface.Stop()
								return serverIp, nodePort, nil
							}
						}
					}
				}
			}
		case <-ctx.Done():
			watchInterface.Stop()
			err := deletePodAndService(podName, serviceName)
			if err != nil {
				return "", 0, fmt.Errorf("pod %s not ready within timeout, error dealing timeout: %w", podName, err)
			}
			return "", 0, fmt.Errorf("pod %s not ready within timeout, pod and service were deleted", podName)
		}
	}
}

func ensureK8sDBConsistency(zoneId string) error {
	// 1. 从集群中获取对应zone下的实例
	podLabelSelector := fmt.Sprintf("zone_id=%s", zoneId)
	podList, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).List(context.Background(), metav1.ListOptions{
		LabelSelector: podLabelSelector,
	})

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
	for _, pod := range podList.Items {
		wg.Add(1)
		ch <- struct{}{}
		go func(pod corev1.Pod) {
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

	total := int32(len(podList.Items))
	if count != total {
		log.Printf("There are %d ready instances in %s totally, and %d instances failed to synchronized", total, zoneId, total-count)
	}
	return nil
}

func checkInstanceStatus(zoneId string, host string, port int32, instanceName string) error {
	status, err := getInstanceStatus(host, port)
	if err != nil {
		return fmt.Errorf("failed to get instance status: %w", err)
	}
	if err := mysql_service.SynchronizeInstanceStatus(zoneId, instanceName, status); err != nil {
		return fmt.Errorf("failed to synchronize instance status for %s: %w", instanceName, err)
	}
	return nil
}

func deletePodAndService(podName string, serviceName string) error {
	var builder strings.Builder
	if err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).Delete(context.Background(), podName, metav1.DeleteOptions{
		GracePeriodSeconds: func() *int64 { t := int64(0); return &t }(),
	}); err != nil {
		builder.WriteString(fmt.Sprintf("error deleting pod %s: %v.", podName, err))
	}

	if err := k8s_client.TargetClient.CoreV1().Services(config.K8SNAMSPACE).Delete(context.Background(), serviceName, metav1.DeleteOptions{}); err != nil {
		builder.WriteString(fmt.Sprintf("error deleting service %s: %v.", serviceName, err))
	}

	if builder.Len() == 0 {
		return nil
	} else {
		return fmt.Errorf(builder.String())
	}
}

func queryCurrentInstanesInCenter(zoneId string) (int, error) {
	podList, err := k8s_client.TargetClient.CoreV1().Pods(config.K8SNAMSPACE).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("zone_id=%s, is_elastic=1", zoneId),
	})
	if err != nil {
		return 0, fmt.Errorf("error listing pods :%v", err)
	}
	return len(podList.Items), nil
}
