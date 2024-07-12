package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var k8sClient *kubernetes.Clientset

func DisconnectAllInstances() {
	podList, err := k8sClient.CoreV1().Pods("cloudgame").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("Failed to get pod list: ", err)
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
	)
	for _, pod := range podList.Items {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()

			serviceName := fmt.Sprintf("service-%s", podName)
			service, err := k8sClient.CoreV1().Services("cloudgame").Get(context.Background(), serviceName, metav1.GetOptions{})
			if err != nil {
				fmt.Printf("Failed to get service: %v\n", err)
				return
			}

			for _, port := range service.Spec.Ports {
				if port.NodePort != 0 {
					if err := sendDisConnectRequest(pod.Status.HostIP, int(port.NodePort)); err != nil {
						fmt.Printf("Failed to disconnect from instance: %v\n", err)
						return
					}
					mu.Lock()
					count++
					mu.Unlock()
					break
				}
			}
		}(pod.Name)
	}

	wg.Wait()
	if int(count) != len(podList.Items) {
		fmt.Printf("%d instances failed to disconnect\n", len(podList.Items)-int(count))
	} else {
		fmt.Println("All instances disconnected successfully")
	}
}

func sendDisConnectRequest(host string, port int) error {
	urlStr := fmt.Sprintf("http://%s:%d/disconnect", host, port)
	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to send connect request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect with status code: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Println(string(body))

	return nil
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(getProjectRoot(), "..", "config", "kubeconfig"))
	if err != nil {
		log.Fatalf("Error creating local Kubernetes config: %v", err)
	}
	config.QPS = 300
	config.Burst = 600
	k8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error connecting local Kubernetes client: %v", err)
	}

	DisconnectAllInstances()
}

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filename
}
