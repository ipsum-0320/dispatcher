package manager

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"predict/mysql"
	"predict/timesnet"
	"runtime"
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

func Calc(predResponse *timesnet.PredDataResponse, zoneId string, siteId string) (int32, error) {
	// 1. 拿到预测值中的最大值。
	maxPred := math.SmallestNonzeroFloat64
	for _, pred := range predResponse.Pred {
		maxPred = math.Max(maxPred, pred)
	}
	// 2.1. 查询当前边缘站点可以容纳多少实例。
	maxSiteInstances, err := queryMaxSiteInstances(zoneId, siteId)
	if err != nil {
		return -1, err
	}
	// 2.2. 查询目前有多少实例跑在边缘站点上。
	siteInstances, err := queryCurrentSiteInstances(zoneId, siteId, false)
	if err != nil {
		return -1, err
	}
	// 2.3. 查询目前有多少实例跑在中心站点上。
	centerInstances, err := queryCurrentSiteInstances(zoneId, siteId, true)
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

func queryMaxSiteInstances(zoneId string, siteId string) (int32, error) {
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT count(*) AS COUNT FROM record_%s WHERE is_elastic = 0 AND site_id = '%s'", zoneId, siteId))
	if err != nil {
		fmt.Printf("%s-%s: query max site instances failed, err:%v\n", zoneId, siteId, err)
		return 0, err
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("%s-%s: close query max site instances failed, err:%v\n", zoneId, siteId, err)
		}
	}(rows)
	var (
		count int32
	)
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			fmt.Printf("%s-%s: scan max site instances failed: %v\n", zoneId, siteId, err)
			return 0, err
		}
	}
	return count, nil
}

func siteOrCloud(isElastic bool) string {
	if isElastic {
		return "cloud"
	} else {
		return "site"
	}
}

func queryCurrentSiteInstances(zoneId string, siteId string, isElastic bool) (int32, error) {
	var isElasticInt int32
	if isElastic {
		isElasticInt = 1
	} else {
		isElasticInt = 0
	}
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT count(*) AS COUNT FROM record_%s WHERE is_elastic = %d AND site_id = '%s' AND status = 'available'", zoneId, isElasticInt, siteId))
	if err != nil {
		fmt.Printf("%s-%s: query current %s instances failed, err:%v\n", zoneId, siteId, siteOrCloud(isElastic), err)
		return 0, err
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("%s-%s: close current %s instances failed, err:%v\n", zoneId, siteId, siteOrCloud(isElastic), err)
		}
	}(rows)
	var (
		count int32
	)
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			fmt.Printf("%s-%s: scan current %s instances failed, err:%v\n", zoneId, siteId, siteOrCloud(isElastic), err)
			return 0, err
		}
	}
	return count, nil
}
