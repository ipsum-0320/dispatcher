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
	c.QPS = 500    // QPS 参数定义了客户端每秒钟可以发送到 API 服务器的最大请求数。这是一个平均值，客户端会尝试不超过这个速率。
	c.Burst = 1000 // Burst 参数定义了在短时间内可以发送到 API 服务器的请求的最大数量，即使这会超过 QPS 设置的速率。当突发请求完成后，客户端将降低请求速率以遵守 QPS 的限制。

	TargetClient, err = kubernetes.NewForConfig(c)
	if err != nil {
		log.Fatalf("Error connecting target Kubernetes config: %v", err)
	}

}
