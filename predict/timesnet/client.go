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
	"predict/config"
	"runtime"
	"sort"
	"strconv"
	"time"
)

var (
	path   = "/predict"
	client = &http.Client{
		Timeout: 600 * time.Second,
	}
)

type PredDataSource map[string]int32

// map { 2024-03-06 00:00:00 => 1023 }

type PredDataResponse struct {
	Length int32
	Pred   []float64
}

func Predict(source PredDataSource, zoneId string, siteId string) (*PredDataResponse, error) {

	// 数据扩大n倍，用于预测
	var scaledPredDataSource = make(PredDataSource)
	for date, value := range source {
		scaledPredDataSource[date] = value * int32(config.SCALERATIO)
	}

	csvPath, err := source2csv(scaledPredDataSource, zoneId, siteId)
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

	url := fmt.Sprintf("%s://%s:%s%s/%s/%s", config.TIMESNETPROTOCOL, config.TIMESNETHOST, config.TIMESNETPORT, path, zoneId, siteId)
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

	// 数据缩小n倍，用于模拟
	for i, value := range responseData.Pred {
		responseData.Pred[i] = value / float64(config.SCALERATIO)
	}
	return &responseData, nil
}

func source2csv(source PredDataSource, zoneId, siteId string) (string, error) {
	_, filename, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(filename)
	csvPath := filepath.Join(curDir, fmt.Sprintf("%s-%s-source.csv", zoneId, siteId))

	// 确保包含文件的目录存在
	dir := filepath.Dir(csvPath)
	err := os.MkdirAll(dir, 0755) // 创建所有必需的父目录，并设置权限
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return "", err
	}
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

	var sourceKeys []string
	for key := range source {
		sourceKeys = append(sourceKeys, key)
	}
	sort.Strings(sourceKeys)

	for _, date := range sourceKeys {
		valueStr := strconv.Itoa(int(source[date]))
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
	return csvPath, nil
}
