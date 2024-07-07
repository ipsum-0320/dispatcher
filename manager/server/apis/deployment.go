package apis

import (
	"encoding/json"
	"fmt"
	"log"
	"manager/config"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/util/uuid"

	mysql_service "manager/mysql/service"
)

type InstanceManageRequest struct {
	ZoneId  string `json:"zone_id"`
	Missing *int32 `json:"missing"`
}

func getRequestData(w http.ResponseWriter, r *http.Request) (InstanceManageRequest, error) {
	reqBody := InstanceManageRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {

		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, err.Error())
		return reqBody, fmt.Errorf("failed to decode request: %v", err)
	}

	if reqBody.Missing == nil || reqBody.ZoneId == "" {
		fmt.Println("Number or zone_id is not specificed")
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusBadRequest,
			ErrorCode:  400,
			Message:    "Bad request",
		}, "Number or zone_id is not specificed")
		return reqBody, fmt.Errorf("zone_id or missing is not specificed")
	}
	return reqBody, nil
}

func InstanceManage(w http.ResponseWriter, r *http.Request) {
	reqBody, err := getRequestData(w, r)
	if err != nil {
		log.Printf("Failed to get request data: %v", err)
		return
	}

	// 获取中心站点可用弹性实例之前刷新一遍实例状态
	log.Println("Check started")
	if err := ensureK8sDBConsistency(reqBody.ZoneId); err != nil {
		log.Printf("Failed to synchronize instance status: %v", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}
	log.Println("Check ended")

	availableInstances, err := mysql_service.GetAvailableInstanceInCenter(reqBody.ZoneId)
	if err != nil {
		log.Printf("Failed to get available instances and delete them: %v", err)
		SendErrorResponse(w, &ErrorCodeWithMessage{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  500,
			Message:    "Internal server error",
		}, err.Error())
		return
	}

	replica := *reqBody.Missing - availableInstances

	log.Println("Manage started")
	defer log.Println("Manage ended")
	if replica == 0 {
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "Replica is 0, there is no need to apply or release instances",
		}, http.StatusOK)
		log.Printf("%s: Replica is 0, there is no need to apply or release instances", reqBody.ZoneId)
	} else if replica > 0 {
		if err := apply(reqBody.ZoneId, replica); err != nil {
			log.Printf("Failed to apply instances: %v", err)
			SendErrorResponse(w, &ErrorCodeWithMessage{
				HttpStatus: http.StatusInternalServerError,
				ErrorCode:  500,
				Message:    "Internal server error",
			}, err.Error())
			return
		}
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "All instances applied successfully",
		}, http.StatusOK)
		log.Printf("%s: All instances applied successfully", reqBody.ZoneId)

	} else {
		if err := release(reqBody.ZoneId, -replica); err != nil {
			log.Printf("Failed to release instances: %v", err)
			SendErrorResponse(w, &ErrorCodeWithMessage{
				HttpStatus: http.StatusInternalServerError,
				ErrorCode:  500,
				Message:    "Internal server error",
			}, err.Error())
			return
		}
		SendHttpResponse(w, &Response{
			StatusCode: 200,
			Message:    "OK",
			Data:       "All instances release successfully",
		}, http.StatusOK)
		log.Printf("%s: %d instances released successfully", reqBody.ZoneId, -replica)
	}
}

func apply(zoneId string, replica int32) error {
	log.Printf("Trying to deploy %d pods in %s", replica, zoneId)
	currentInstancesNumber, err := queryCurrentInstanesInCenter(zoneId)
	if err != nil {
		return fmt.Errorf("error quering current instances in zone %s: %w", zoneId, err)
	}
	if currentInstancesNumber >= config.CENTERMAXTOTAL {
		return fmt.Errorf("the number of elastic instance in zone %s is already full", zoneId)
	}
	leftNumber := int32(config.CENTERMAXTOTAL) - int32(currentInstancesNumber)
	if leftNumber < replica {
		replica = leftNumber
		log.Printf("But the left space can only deploy %d pods in %s", replica, zoneId)
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
		ch    = make(chan struct{}, 50)
	)
	for i := 1; i <= int(replica); i++ {
		wg.Add(1)
		ch <- struct{}{}
		go func(zoneId string) {
			defer wg.Done()
			defer func() { <-ch }()

			randomId := uuid.NewUUID()
			podName := fmt.Sprintf("cloudgame-center-%s", randomId)
			instanceId := fmt.Sprintf("instance-%s", podName)

			serverIp, port, err := createAndWatchPod(podName, instanceId, zoneId)
			if err != nil {
				log.Printf("Failed to create and watch Pod: %v", err)
				return
			}

			// 部署完成后才能保存实例到数据库
			if err = mysql_service.InsertInstance(zoneId, "null", serverIp, instanceId, podName, port, 1, "available", "null"); err != nil {
				log.Printf("Failed to insert instance into database  when applying: %v", err)
				return
			}

			// 计算成功部署的实例数量
			mu.Lock()
			count++
			mu.Unlock()
		}(zoneId)
	}

	wg.Wait()

	if count != replica {
		return fmt.Errorf("%d instances need to be applied, but %d instances failed to apply", replica, replica-count)
	}

	return nil
}

func release(zoneId string, replica int32) error {

	podList, err := mysql_service.GetAndDeleteAvailableInstancesInCenter(zoneId, replica)
	if err != nil {
		return fmt.Errorf("failed to get available instances from database")
	} else if podList == nil {
		return fmt.Errorf("there is no elastic instance in this zone")
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = int32(0)
	)
	for _, podName := range podList {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()

			serviceName := fmt.Sprintf("service-%s", podName)
			err := deletePodAndService(podName, serviceName)
			if err != nil {
				log.Printf("Failed to release pod %s: %v", podName, err)
				return
			}

			mu.Lock()
			count++
			mu.Unlock()
		}(podName)
	}

	wg.Wait()

	if count != replica {
		return fmt.Errorf("%d instances need to be released, but %d instances failed to release", replica, replica-count)
	}

	return nil
}
