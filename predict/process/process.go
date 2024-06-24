package process

import (
	"fmt"
	"math"
	"predict/manager"
	"predict/mysql"
	mysqlservice "predict/mysql/service"
	"predict/timesnet"
	"sort"
	"sync"
	"time"
)

var (
	TEnd   *time.Time = nil
	TStart *time.Time = nil
	layout            = "2006-01-02 15:04:05"
)

func Process(zoneId string, siteList []string) error {
	zoneMissing := int32(0)
	zonePredInstance := float64(0)

	latestTime := time.Date(2001, 1, 1, 0, 0, 0, 0, time.Local)
	siteDateTrueInstanceMap := make(map[string]map[string]int32)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, siteId := range siteList {
		wg.Add(1)
		go func(zoneId string, siteId string) {
			defer wg.Done()
			queryDateInstanceSQL := fmt.Sprintf("SELECT site_id, date, instances, login_failures FROM record_%s WHERE site_id = '%s' ORDER BY date DESC LIMIT 180", zoneId, siteId)
			DateInstanceRows, err := mysql.DB.Query(queryDateInstanceSQL)
			if err != nil {
				fmt.Printf("%s-%s, query date instance failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s, query date instance failed, err:%v\n", zoneId, siteId, err))
			}
			predMap := make(timesnet.PredDataSource)
			for DateInstanceRows.Next() {
				var (
					siteId         string
					date           string
					instances      int32
					login_failures int32
				)
				if err := DateInstanceRows.Scan(&siteId, &date, &instances, &login_failures); err != nil {
					fmt.Printf("%s-%s: scan date instance failed: %v\n", zoneId, siteId, err)
					panic(fmt.Sprintf("%s-%s: scan date instance failed: %v\n", zoneId, siteId, err))
				}
				dateTime, err := time.ParseInLocation(layout, date, time.Local)
				if err != nil {
					fmt.Printf("%s-%s: parse date failed: %v\n", zoneId, siteId, err)
					panic(fmt.Sprintf("%s-%s: parse date failed: %v\n", zoneId, siteId, err))
				}
				fmt.Printf("latestTime: %v, dateTime: %v\n", latestTime, dateTime)
				if latestTime.Before(dateTime) {
					latestTime = dateTime
				}
				predMap[date] = instances + login_failures
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
			siteDateTrueInstanceMap[siteId] = predMap
			predResponse, err := timesnet.Predict(predMap, zoneId, siteId)
			if err != nil {
				fmt.Printf("%s-%s: predict failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: predict failed, err:%v\n", zoneId, siteId, err))
			}

			maxPred := math.SmallestNonzeroFloat64
			for _, pred := range predResponse.Pred {
				maxPred = math.Max(maxPred, pred)
			}
			siteMissing, err := manager.CalculateMissingInstancesForSite(maxPred, zoneId, siteId)
			if err != nil {
				fmt.Printf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err)
				panic(fmt.Sprintf("%s-%s: calc failed, err:%v\n", zoneId, siteId, err))
			}
			mu.Lock()
			zoneMissing += siteMissing
			zonePredInstance += maxPred
			mu.Unlock()
		}(zoneId, siteId)
	}
	wg.Wait()

	dateInstanceMap := make(map[string]int32)
	// 默认是 kv 零值。
	for _, DI := range siteDateTrueInstanceMap {
		for date, instance := range DI {
			dateInstanceMap[date] += instance
		}
	}

	// 将其存储到数据库中。
	// 存储之前，应当排序。
	var sourceKeys []string
	for key := range dateInstanceMap {
		sourceKeys = append(sourceKeys, key)
	}
	sort.Strings(sourceKeys)

	for _, date := range sourceKeys {
		// 插入之前需要检查一下是否已经插入过了。
		isExist, err := mysqlservice.QueryBounceRecordExist(zoneId, date)
		if err != nil {
			fmt.Printf("%s: query bounce record exist failed, err: %v\n", zoneId, err)
			return err
		}
		if isExist {
			continue
		}
		err = mysqlservice.InsertBounceRecord(zoneId, date, dateInstanceMap[date])
		if err != nil {
			fmt.Printf("%s: insert true instances into bounce record failed, err: %v\n", zoneId, err)
			return err
		}
	}

	if TStart != nil {
		// 插入最新的值。
		TEnd = &latestTime
		var timeStrings []string
		for t := *TStart; t.Before(*TEnd) || t.Equal(*TEnd); t = t.Add(1 * time.Minute) {
			formattedTime := t.Format(layout)
			timeStrings = append(timeStrings, formattedTime)
		}
		fmt.Printf("TODO: TStart: %s, TEnd: %s, timeStrings: %v \n", TStart.Format(layout), TEnd.Format(layout), timeStrings)
		for _, timeString := range timeStrings {
			err := mysqlservice.UpdateBounceRecord(zoneId, timeString, int32(zonePredInstance))
			if err != nil {
				fmt.Printf("%s: update pred instance into bounce record failed, err: %v\n", zoneId, err)
			}
		}
	}
	newStart := latestTime.Add(1 * time.Minute)
	TStart = &newStart
	if err := manager.Manage(zoneId, zoneMissing); err != nil {
		fmt.Printf("Failed to apply or release instances in %s center: %v\n", zoneId, err)
		return err
	}
	return nil
}
