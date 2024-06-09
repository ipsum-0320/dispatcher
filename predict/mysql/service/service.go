package service

import (
	"database/sql"
	"fmt"
	"log"
	"predict/mysql"
	"strings"
)

func GetZoneListInDB() (map[string][]string, error) {
	rows, err := mysql.DB.Query("SHOW TABLES LIKE 'instance_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	zoneList := make(map[string][]string)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		ZoneID := strings.TrimPrefix(tableName, "instance_")

		sites, err := GetSiteListInZone(ZoneID)
		if err != nil {
			log.Printf("Error getting unique site IDs for %s: %v", tableName, err)
			continue
		}
		zoneList[ZoneID] = sites
	}

	fmt.Println(zoneList)
	return zoneList, nil
}

func GetSiteListInZone(ZoneID string) ([]string, error) {
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT site_id FROM instance_%s WHERE site_id != 'null'", ZoneID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var siteList []string
	for rows.Next() {
		var siteID string
		if err := rows.Scan(&siteID); err != nil {
			return nil, err
		}
		siteList = append(siteList, siteID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return siteList, nil
}

func QueryMaxSiteInstances(zoneId string, siteId string) (int32, error) {
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT count(*) AS COUNT FROM instance_%s WHERE is_elastic = 0 AND site_id = '%s'", zoneId, siteId))
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

func QueryCurrentSiteInstances(zoneId string, siteId string, isElastic bool) (int32, error) {
	var isElasticInt int32
	if isElastic {
		isElasticInt = 1
	} else {
		isElasticInt = 0
	}
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT count(*) AS COUNT FROM instance_%s WHERE is_elastic = %d AND site_id = '%s' AND status = 'available'", zoneId, isElasticInt, siteId))
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

func InsertBounceRecord(zoneId string, date string, trueIns int32) error {
	query := fmt.Sprintf("INSERT INTO bounce_%s (date, true_instances) VALUES (?, ?)", zoneId)
	stmt, err := mysql.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(date, trueIns)
	if err != nil {
		return err
	}
	return nil
}

func UpdateBounceRecord(zoneId string, date string, predIns int32) error {
	query := fmt.Sprintf("UPDATE bounce_%s SET pred_instances = ? WHERE date = ?", zoneId)
	stmt, err := mysql.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(date, predIns)
	if err != nil {
		return err
	}
	return nil
}

func QueryCenterInstances(zoneId string) (int32, error) {
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT count(*) AS COUNT FROM instance_%s WHERE is_elastic = 1", zoneId))
	if err != nil {
		fmt.Printf("%s: query center instances failed, err:%v\n", zoneId, err)
		return 0, err
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("%s: close center instances failed, err:%v\n", zoneId, err)
		}
	}(rows)
	var (
		count int32
	)
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			fmt.Printf("%s: scan center instances failed, err:%v\n", zoneId, err)
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
