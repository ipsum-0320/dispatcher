package k8s_client

import (
	"log"
	"manager/config"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var TargetClient *kubernetes.Clientset
var LocalClient *kubernetes.Clientset

func init() {

	// 从业务集群中获取 config，并创建客户端。
	c, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating local Kubernetes config: %v", err)
	}
	LocalClient, err = kubernetes.NewForConfig(c)
	if err != nil {
		log.Fatalf("Error connecting local Kubernetes client: %v", err)
	}

	// 从环境变量获取目标集群config，并创建客户端
	c, err = clientcmd.BuildConfigFromFlags("", config.K8SCONFIGPATH)
	if err != nil {
		log.Fatalf("Error creating target Kubernetes config: %v", err)
	}

	TargetClient, err = kubernetes.NewForConfig(c)
	if err != nil {
		log.Fatalf("Error connecting target Kubernetes config: %v", err)
	}

}
