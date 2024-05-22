package main

import (
	"fmt"
	"predict/manager"
	"predict/mysql"
	"predict/timesnet"
	"sync"

	mysql_service "predict/mysql/service"
)

func Process(zoneId string, siteList []string) error {
	zoneApply := int32(0)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, siteId := range siteList {
		wg.Add(1)
		go func(zoneId string, siteId string) {
			defer wg.Done()
			queryDateInstanceSQL := fmt.Sprintf("SELECT id, site_id, date, instances FROM record_%s WHERE site_id = '%s'", zoneId, siteId)
			DateInstanceRows, err := mysql.DB.Query(queryDateInstanceSQL)
			if err != nil {
				fmt.Printf("%s-%s, query date instance failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s, query date instance failed, err:%v\n", zoneId, siteId, err))
			}
			predMap := make(timesnet.PredDataSource)
			for DateInstanceRows.Next() {
				var (
					id        int64
					siteId    string
					date      string
					instances int32
				)
				if err := DateInstanceRows.Scan(&id, &siteId, &date, &instances); err != nil {
					fmt.Printf("%s-%s: scan date instance failed: %v\n", zoneId, siteId, err)
					panic(fmt.Sprintf("%s-%s: scan date instance failed: %v\n", zoneId, siteId, err))
				}
				predMap[date] = instances
			}
			if err := DateInstanceRows.Err(); err != nil {
				fmt.Printf("%s-%s: error during date instance iteration: %v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: error during date instance iteration: %v\n", zoneId, siteId, err))
			}
			err = DateInstanceRows.Close()
			if err != nil {
				fmt.Printf("%s-%s: close query date instance failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: close query date instance failed, err:%v\n", zoneId, siteId, err))
			}

			predResponse, err := timesnet.Predict(predMap, siteId)
			if err != nil {
				fmt.Printf("%s-%s: predict failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: predict failed, err:%v\n", zoneId, siteId, err))
			}
			siteApply, err := manager.CalculateApplyNumberForSite(predResponse, zoneId, siteId)
			if err != nil {
				fmt.Printf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err))
			}
			mu.Lock()
			zoneApply += siteApply
			mu.Unlock()
		}(zoneId, siteId)
	}
	wg.Wait()

	centerAvailable, err := mysql_service.GetAvailableInstanceInCenter(zoneId)
	if err != nil {
		fmt.Printf("Failed to get available instances in %s center: %v\n", zoneId, err)
		return err
	}

	if err = manager.Manage(zoneId, zoneApply-centerAvailable); err != nil {
		fmt.Printf("Failed to apply or release instances in %s center: %v\n", zoneId, err)
		return err
	}
	return nil
}
