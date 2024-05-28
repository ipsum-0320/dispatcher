package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"predict/config"
	mysql_service "predict/mysql/service"
	"predict/timesnet"
)

func AbsInt(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func CalculateApplyNumberForSite(predResponse *timesnet.PredDataResponse, zoneId string, siteId string) (int32, error) {
	// 1. 拿到预测值中的最大值。
	maxPred := math.SmallestNonzeroFloat64
	for _, pred := range predResponse.Pred {
		maxPred = math.Max(maxPred, pred)
	}
	// 2.1. 查询当前边缘站点可以容纳多少实例。
	maxSiteInstances, err := mysql_service.QueryMaxSiteInstances(zoneId, siteId)
	if err != nil {
		return -1, err
	}
	// 2.2. 查询目前有多少实例跑在边缘站点上。
	siteInstances, err := mysql_service.QueryCurrentSiteInstances(zoneId, siteId, false)
	if err != nil {
		return -1, err
	}
	// 2.3. 查询目前有多少实例跑在中心站点上。
	centerInstances, err := mysql_service.QueryCurrentSiteInstances(zoneId, siteId, true)
	if err != nil {
		return -1, err
	}
	// 2.4. 计算预计还缺少的资源的实例有多少。
	unAllocateInstances := int32(maxPred - float64(siteInstances+centerInstances))
	// 2.5. 计算边缘站点还有多少容量可以利用。
	capacitySiteInstances := maxSiteInstances - siteInstances
	// 2.6. 只有当预测到实例增加，且边缘站点空闲实例数不足以支撑时，才需要额外申请弹性实例。
	if unAllocateInstances >= 0 && capacitySiteInstances < unAllocateInstances {
		return int32(unAllocateInstances - capacitySiteInstances), nil
	}

	return 0, nil
}

func Manage(zoneId string, replica int32) error {
	var path string
	if replica == 0 {
		fmt.Printf("Replica is 0, no need to apply or release instances\n")
		return nil
	} else if replica > 0 {
		path = "/instance/apply"
		fmt.Printf("%d instances need to be applied", replica)
	} else {
		path = "/instance/release"
		replica = -replica
		fmt.Printf("%d instances need to be released", replica)
	}

	url := fmt.Sprintf("%s://%s:%s%s", config.MANAGERPROTOCOL, config.MANAGERHOST, config.MANAGERPORT, path)

	requestBodyData := map[string]interface{}{
		"zone_id": zoneId,
		"number":  replica,
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
