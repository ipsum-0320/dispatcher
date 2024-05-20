package main

import (
	"database/sql"
	"fmt"
	"predict/manager"
	"predict/mysql"
	"predict/timesnet"
	"sync"
)

func Process(zoneId string) error {
	querySiteIdSQL := fmt.Sprintf("SELECT DISTINCT site_id FROM record_%s", zoneId)
	siteIdRows, err := mysql.DB.Query(querySiteIdSQL)
	if err != nil {
		fmt.Printf("%s: query siteId failed, err:%v\n", zoneId, err)
		return err
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("%s: close query siteId failed, err:%v\n", zoneId, err)
		}
	}(siteIdRows)
	siteIds := make([]string, 0)
	for siteIdRows.Next() {
		var siteId string
		if err := siteIdRows.Scan(&siteId); err != nil {
			fmt.Printf("%s: scan siteId failed: %v\n", zoneId, err)
			return err
		}
		siteIds = append(siteIds, siteId)
	}
	if err := siteIdRows.Err(); err != nil {
		fmt.Printf("%s, error during siteId iteration: %v\n", zoneId, err)
		return err
	}

	replica := int32(0)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, siteId := range siteIds {
		wg.Add(1)
		go func(zoneId string, siteId string) {
			defer wg.Done()
			queryDateInstanceSQL := fmt.Sprintf("SELECT id, site_id, date, instances FROM record_%s WHERE site_id = %s", zoneId, siteId)
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
			calc, err := manager.Calc(predResponse, zoneId, siteId)
			if err != nil {
				fmt.Printf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err))
			}
			mu.Lock()
			replica += calc
			mu.Unlock()
		}(zoneId, siteId)
	}
	wg.Wait()
	err = manager.Manage(zoneId, replica)
	return nil
}
