package process

import (
	"fmt"
	"predict/manager"
	"predict/mysql"
	"predict/timesnet"
	"sync"
)

func Process(zoneId string, siteList []string) error {
	zoneMissing := int32(0)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, siteId := range siteList {
		wg.Add(1)
		go func(zoneId string, siteId string) {
			defer wg.Done()
			queryDateInstanceSQL := fmt.Sprintf("SELECT site_id, date, instances FROM record_%s WHERE site_id = '%s' ORDER BY date DESC LIMIT 180", zoneId, siteId)
			DateInstanceRows, err := mysql.DB.Query(queryDateInstanceSQL)
			if err != nil {
				fmt.Printf("%s-%s, query date instance failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s, query date instance failed, err:%v\n", zoneId, siteId, err))
			}
			predMap := make(timesnet.PredDataSource)
			for DateInstanceRows.Next() {
				var (
					siteId    string
					date      string
					instances int32
				)
				if err := DateInstanceRows.Scan(&siteId, &date, &instances); err != nil {
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
			if len(predMap) != 180 {
				fmt.Printf("%s-%s: date instance length is not 180\n", zoneId, siteId)
				return
			}

			predResponse, err := timesnet.Predict(predMap, zoneId, siteId)
			if err != nil {
				fmt.Printf("%s-%s: predict failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: predict failed, err:%v\n", zoneId, siteId, err))
			}
			siteMissing, err := manager.CalculateMissingInstancesForSite(predResponse, zoneId, siteId)
			if err != nil {
				fmt.Printf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err))
			}
			mu.Lock()
			zoneMissing += siteMissing
			mu.Unlock()
		}(zoneId, siteId)
	}
	wg.Wait()

	if err := manager.Manage(zoneId, zoneMissing); err != nil {
		fmt.Printf("Failed to apply or release instances in %s center: %v\n", zoneId, err)
		return err
	}
	return nil
}
