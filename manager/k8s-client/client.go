package k8s_client

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"path/filepath"
	"runtime"
)

var Client *kubernetes.Clientset

func init() {
	_, filename, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(filename)
	configPath := filepath.Join(curDir, "..", "conf", "config")
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println("connect failed")
	}

	Client = clientset
}
