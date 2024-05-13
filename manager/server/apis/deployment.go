package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	k8s_client "manager/k8s-client"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	namespace = "default"
)

func deploymentFactory(
	name string,
	zoneId string,
) *appsv1.Deployment {
	defaultDeploymentReplicas := int32(0)

	defaultDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"Zone_id": zoneId,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &defaultDeploymentReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"Zone_id": zoneId,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"Zone_id": zoneId,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "huawei-game-instance",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
	return defaultDeployment
}

type DeploymentApplyRequest struct {
	DeploymentName string `json:"deploymentName"`
	ZoneId         string `json:"zoneId"`
	Replica        int32  `json:"replica"`
}

func DeploymentApply(w http.ResponseWriter, r *http.Request) {
	// 基于 deployment 的 replica 机制。
	reqBody := DeploymentApplyRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		fmt.Printf("Failed to decode request: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, err.Error())
		return
	}

	deploymentName := reqBody.DeploymentName
	dp, err := k8s_client.Client.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get deployment: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}
	if dp == nil {
		// 不存在，创建 deployment。
		deployment := deploymentFactory(deploymentName, reqBody.ZoneId)
		_, err = k8s_client.Client.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			fmt.Printf("Failed to create deployment: %v\n", err)
			SendErrorResponse(w, &ErrorCodeWithMessage{
				HttpStatus: http.StatusInternalServerError,
				ErrorCode:  500,
				Message:    "Internal server error",
			}, err.Error())
			return
		}
	}

	// 更新 deployment 的 replica。
	patchReplicas := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, reqBody.Replica))
	_, err = k8s_client.Client.AppsV1().Deployments(namespace).Patch(context.TODO(), deploymentName, types.MergePatchType,
		patchReplicas, metav1.PatchOptions{})
	if err != nil {
		fmt.Printf("Failed to patch deployment: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	// 返回成功结果。
	SendHttpResponse(w, &Response{
		StatusCode: 200,
		Message:    "OK",
		Data:       "Deployment applied successfully",
	}, http.StatusOK)
}

type DeploymentPodWatchResponse struct {
	ReadyNum   int32 `json:"readyNum"`
	UnReadyNum int32 `json:"unReadyNum"`
}

func DeploymentPodWatch(w http.ResponseWriter, r *http.Request) {
	// 通过 watch 的方式监听 deployment 下的 pod。
	deploymentName := r.URL.Query().Get("deploymentName")
	zoneId := r.URL.Query().Get("zoneId")
	if len(deploymentName) == 0 || len(zoneId) == 0 {
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, "Deployment name not specified")
		return
	}

	dp, err := k8s_client.Client.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get deployment: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}
	replica := dp.Spec.Replicas
	// 通过 label selector 来筛选 pod。
	pods, err := k8s_client.Client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("Zone_id=%s", zoneId),
	})
	if err != nil {
		fmt.Printf("Failed to list pods: %v\n", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	var readyNum int32
	for _, pod := range pods.Items {
		if isReady := isPodReady(&pod); isReady {
			readyNum++
		}
	}

	respBody := &DeploymentPodWatchResponse{
		ReadyNum:   readyNum,
		UnReadyNum: *replica - readyNum,
	}

	SendHttpResponse(w, &Response{
		StatusCode: 200,
		Message:    "OK",
		Data:       respBody,
	}, http.StatusOK)
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
