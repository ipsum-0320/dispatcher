package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"predict/config"
	mysql_service "predict/mysql/service"
)

func AbsInt(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func CalculateMissingInstancesForSite(maxPred float64, zoneId string, siteId string) (int32, error) {
	// 1. 查询当前边缘站点的容量。
	siteCapacity, err := mysql_service.QuerySiteCapacity(zoneId, siteId)
	if err != nil {
		return -1, err
	}
	// 2. 查询目前有多少实例跑在边缘站点上。
	siteUsingInstances, err := mysql_service.QueryUsingInstances(zoneId, siteId, "site")
	if err != nil {
		return -1, err
	}
	// 3. 查询目前有多少实例跑在中心站点上。
	centerUsingInstances, err := mysql_service.QueryUsingInstances(zoneId, siteId, "center")
	if err != nil {
		return -1, err
	}
	// 4. 计算预计还缺少的资源的实例有多少。
	unAllocateInstances := int32(maxPred - float64(siteUsingInstances+centerUsingInstances))
	// 5. 计算边缘站点还有多少容量可以利用。
	siteAvailableInstances := siteCapacity - siteUsingInstances
	// 6. 只有当预测到实例增加，且边缘站点空闲实例数不足以支撑时，才需要额外的弹性实例。
	if unAllocateInstances >= 0 && siteAvailableInstances < unAllocateInstances {
		return int32(unAllocateInstances - siteAvailableInstances), nil
	}

	return 0, nil
}

// zoneId: 区域id
// missing: 该zone各个边缘缺少的实例总量
func Manage(zoneId string, missing int32) error {
	var path = "/instance/manage"

	url := fmt.Sprintf("%s://%s:%s%s", config.MANAGERPROTOCOL, config.MANAGERHOST, config.MANAGERPORT, path)

	requestBodyData := map[string]interface{}{
		"zone_id": zoneId,
		"missing": missing,
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
		fmt.Printf("Failed to apply or release instances: %v\n", response.Status)
		return fmt.Errorf("failed to apply or release instances: %v", response.Status)
	}
	return nil
}
