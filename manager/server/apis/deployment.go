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
