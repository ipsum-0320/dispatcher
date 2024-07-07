package apis

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Response struct {
	StatusCode uint32      `json:"status_code"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
}

type ErrorCodeWithMessage struct {
	HttpStatus int
	ErrorCode  uint32
	Message    string
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

func SendErrorResponse(w http.ResponseWriter, err *ErrorCodeWithMessage, describe string) {
	SendHttpResponse(w, &Response{
		StatusCode: err.ErrorCode,
		Message:    err.Message,
		Data:       describe,
	}, err.HttpStatus)
}

func getInstanceStatus(host string, port int32) (string, error) {
	url := fmt.Sprintf("http://%s:%d/getStatus", host, port)
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("error with request: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}
	return string(body), nil
}

func checkPodReady(host string, port int32) bool {
	url := fmt.Sprintf("http://%s:%d/healthz", host, port)
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	for i := 0; i < 3; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}
		time.Sleep(5 * time.Second)
	}
	return false
}
