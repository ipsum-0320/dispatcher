package timesnet

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// python server 不在 k8s 部署，因此可以硬编码配置。
var (
	protocol = "http"
	host     = "localhost"
	port     = 5000
	path     = "/predict"
	client   = &http.Client{
		Timeout: 20 * time.Second, // 设置超时时间为10秒
	}
)

type PredDataSource map[string]int32

// map { 2024-03-06 00:00:00 => 1023 }

type PredDataResponse struct {
	Length int32
	Pred   []float64
}

func Predict(source PredDataSource, siteId string) (*PredDataResponse, error) {
	csvPath, err := source2csv(source)
	if err != nil {
		fmt.Println("Error converting source to CSV:", err)
		return nil, err
	}

	var reqBody bytes.Buffer
	writer := multipart.NewWriter(&reqBody)

	file, err := os.Open(csvPath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("Error closing file:", err)
		}
	}(file)

	fileWriter, err := writer.CreateFormFile("source", csvPath)
	if err != nil {
		fmt.Println("Error creating form file:", err)
		return nil, err
	}
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		fmt.Println("Error copying file to form:", err)
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		fmt.Println("Error closing writer:", err)
		return nil, err
	}

	url := fmt.Sprintf("%s://%s:%d%s/%s", protocol, host, port, path, siteId)
	// 对于每个边缘站点的预测，都会有一个对应的请求路径，siteId 用作区分。
	req, err := http.NewRequest("POST", url, &reqBody)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)

	var responseData PredDataResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&responseData); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return nil, err
	}
	return &responseData, nil
}

func source2csv(source PredDataSource) (string, error) {
	_, filename, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(filename)
	csvPath := filepath.Join(curDir, "source.csv")
	file, err := os.OpenFile(csvPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return "", err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("Error closing file:", err)
		}
	}(file)

	writer := csv.NewWriter(file)
	err = writer.Write([]string{"date", "value"})
	if err != nil {
		fmt.Println("Error writing CSV:", err)
		return "", err
	}

	for date, value := range source {
		valueStr := strconv.Itoa(int(value))
		err := writer.Write([]string{date, valueStr})
		if err != nil {
			fmt.Println("Error writing CSV:", err)
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Println("Error writing CSV:", err)
		return "", err
	}
	fmt.Println("CSV file overwritten successfully.")
	return csvPath, nil
}
