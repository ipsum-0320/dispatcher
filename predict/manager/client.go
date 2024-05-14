package manager

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"predict/mysql"
	"predict/timesnet"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// manager 通过 k8s 部署，因此最好使用 configmap 获取 manager 的相关配置。
var (
	protocol    = "http"
	managerIP   string
	managerPort int32
)

type Config struct {
	ip   string
	port int32
}

func init() {
	// 从配置文件中读取 manager 的相关配置。
	_, filename, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(filename)
	managerConfigPath := filepath.Join(curDir, "manager.yaml")
	yamlFile, err := os.ReadFile(managerConfigPath)
	if err != nil {
		fmt.Printf("Failed to read YAML: %v", err)
		return
	}
	var managerConfig Config
	if err := yaml.Unmarshal(yamlFile, &managerConfig); err != nil {
		fmt.Printf("Failed to unmarshal YAML: %v", err)
		return
	}

	managerIP = managerConfig.ip
	managerPort = managerConfig.port
}

func AbsInt(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func Calc(predResponse *timesnet.PredDataResponse, siteId string) (int32, error) {
	// 1. 拿到预测值中的最大值。
	maxPred := math.SmallestNonzeroFloat64
	for _, pred := range predResponse.Pred {
		maxPred = math.Max(maxPred, pred)
	}
	// 2.1. 查询当前边缘站点可以容纳多少实例。
	maxSiteInstances, err := queryMaxSiteInstances(siteId)
	if err != nil {
		return -1, err
	}
	// 2.2. 查询目前有多少实例跑在边缘站点上。
	siteInstances, err := queryCurrentSiteInstances(siteId, false)
	if err != nil {
		return -1, err
	}
	// 2.3. 查询目前有多少实例跑在中心站点s上。
	centerInstances, err := queryCurrentSiteInstances(siteId, true)
	if err != nil {
		return -1, err
	}
	// 2.4. 计算预计还缺少的资源的实例有多少。
	unAllocateInstances := int32(maxPred - float64(siteInstances+centerInstances))
	// 2.5. 计算边缘站点还有多少容量可以利用。
	capacitySiteInstances := maxSiteInstances - siteInstances
	// 2.6. 计算需要向中心云站点申请多少资源。
	needCenterInstances := unAllocateInstances - capacitySiteInstances
	if needCenterInstances <= 0 {
		// 计算剩余的边缘站点容量。
		leftCapacity := AbsInt(needCenterInstances)
		// 计算是否还需要中心站点资源，如果需要，还需要多少。
		newCenter := leftCapacity - centerInstances
		if newCenter >= 0 {
			// newCenter >= 0 意味着不需要。
			return 0, nil
		}
		// newCenter < 0 意味着需要。
		return AbsInt(newCenter), nil
	} else {
		// 需要原来已有的 + 额外申请的中心站点资源。
		return needCenterInstances + centerInstances, nil
	}
}

func Manage(zoneId string, replica int32) error {
	if replica < 0 {
		// 如果设置为负数，那么 replica 直接归为零值就好。
		replica = 0
	}
	err := apply(zoneId+"-deploy", zoneId, replica)
	if err != nil {
		fmt.Printf("Failed to apply deployment: %v\n", err)
		return err
	}
	loopCount := 0
	for true {
		watchRes, err := podWatch(zoneId+"-deploy", zoneId)
		if err != nil {
			fmt.Printf("Failed to watch deployment pod: %v\n", err)
			return err
		}
		if watchRes.UnReadyNum == 0 {
			break
		}
		fmt.Printf("Deployment pod not ready, readyNum: %d/%d\n", watchRes.ReadyNum, watchRes.ReadyNum+watchRes.UnReadyNum)
		loopCount++
		if loopCount >= 10 {
			fmt.Printf("Failed to watch deployment pod: timeout\n")
			return fmt.Errorf("failed to watch deployment pod: timeout, retry 10")
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

func apply(deploymentName, zoneId string, replica int32) error {
	path := "deployment/apply"
	url := fmt.Sprintf("%s://%s:%d%s", protocol, managerIP, managerPort, path)

	requestBodyData := map[string]interface{}{
		"deploymentName": deploymentName,
		"zoneId":         zoneId,
		"replica":        replica,
	}

	jsonData, err := json.Marshal(requestBodyData)
	if err != nil {
		fmt.Printf("Failed to marshal request body data: %v\n", err)
		return err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		fmt.Printf("Error occurred while sending request. Error: %v\n", err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}(response.Body)

	if response.StatusCode != http.StatusOK {
		fmt.Printf("Failed to apply deployment: %v\n", response.Status)
		return fmt.Errorf("failed to apply deployment: %v", response.Status)
	}

	return nil
}

type DeploymentPodWatchResponse struct {
	ReadyNum   int32 `json:"readyNum"`
	UnReadyNum int32 `json:"unReadyNum"`
}

func podWatch(deploymentName, zoneId string) (*DeploymentPodWatchResponse, error) {
	path := "/deployment/pod/watch"
	url := fmt.Sprintf("%s://%s:%d%s", protocol, managerIP, managerPort, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return nil, err
	}

	params := req.URL.Query()
	params.Add("deploymentName", deploymentName)
	params.Add("zoneId", zoneId)
	req.URL.RawQuery = params.Encode()
	req.Header.Set("Content-Type", "application/json")

	resp, _ := http.DefaultClient.Do(req)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to watch deployment pod: %v\n", resp.Status)
		return nil, fmt.Errorf("failed to watch deployment pod: %v", resp.Status)
	}

	var responseData DeploymentPodWatchResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&responseData); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return nil, err
	}
	return &responseData, nil
}

func queryMaxSiteInstances(siteId string) (int32, error) {
	rows, err := mysql.DB.Query("")
	if err != nil {
		fmt.Printf("query max site instances failed, err:%v\n", err)
		return 0, err
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("close query max site instances failed, err:%v\n", err)
		}
	}(rows)
	var (
		id        int64
		siteIdDB  string
		instances int32
	)
	if rows.Next() {
		if err := rows.Scan(&id, &siteIdDB, &instances); err != nil {
			fmt.Printf("scan max site instances failed: %v\n", err)
			return 0, err
		}
	}
	return instances, nil
}

func queryCurrentSiteInstances(siteId string, isElastic bool) (int32, error) {
	return -1, nil
}
