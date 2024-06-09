package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"net/http"
)

var k8sClient *kubernetes.Clientset

func DisconnectAllInstances() {
	podList, err := k8sClient.CoreV1().Pods("cloudgame").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("Failed to get pod list: ", err)
	}

	for _, pod := range podList.Items {
		serviceName := fmt.Sprintf("service-%s", pod.Name)
		service, err := k8sClient.CoreV1().Services("cloudgame").Get(context.Background(), serviceName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Failed to get service: %v\n", err)
			continue
		}

		for _, port := range service.Spec.Ports {
			if port.NodePort != 0 {
				if err := sendDisConnectRequest(pod.Status.HostIP, int(port.NodePort)); err != nil {
					fmt.Printf("Failed to disconnect from instance: %v\n", err)
				}
				break
			}
		}
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

type Response struct {
	StatusCode uint32      `json:"status_code"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
}

func SendHttpResponse(w http.ResponseWriter, m *Response, httpCode int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")

	w.WriteHeader(httpCode)
	if m != nil {
		err := json.NewEncoder(w).Encode(m)
		if err != nil {
			fmt.Printf("Failed to encode response: %v\n", err)
			return
		}
	}
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", "k8s.config")
	client, err := kubernetes.NewForConfig(config)
	k8sClient = client
	router := mux.NewRouter().StrictSlash(true)
	router.
		Methods(http.MethodGet).
		Path("/disconnectAll").
		Name("bounceRate").
		HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			DisconnectAllInstances()
			SendHttpResponse(writer, &Response{
				StatusCode: 200,
				Message:    "OK",
				Data:       "disconnect all instances",
			}, http.StatusOK)
		})
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("localhost:9988"),
		Handler: router,
	}
	klog.Info("server start...")
	err = httpServer.ListenAndServe()
	if err != nil {
		return
	}
}
