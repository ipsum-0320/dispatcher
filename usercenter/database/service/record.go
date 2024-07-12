package service

import (
	"fmt"
	"time"
	"usercenter/database"
)

// RecordCountForSite 查询站点下 status 为 'using' 的实例个数
func RecordCountForSite(zoneID string, siteID string) (int, error) {

	query := fmt.Sprintf("SELECT COUNT(*) FROM instance_%s WHERE site_id = ? AND status = 'using'", zoneID)
	var count int
	err := database.DB.QueryRow(query, siteID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// InsertRecord 插入记录到 records 表
func InsertRecord(zoneID string, siteID string, date string, instances int, loginFailures int) error {
	insertQuery := fmt.Sprintf("INSERT INTO record_%s (site_id, date, instances, login_failures) VALUES (?, ?, ?, ?)", zoneID)
	if _, err := database.DB.Exec(insertQuery, siteID, date, instances, loginFailures); err != nil {
		return err
	}

	return nil
}

// QueryLoginFailures 查询某个 Site 过去一段时间内登陆失败的次数
func QueryLoginFailures(zoneID string, siteID string, endTime time.Time, duration time.Duration) (int, error) {
	var count int
	startTime := endTime.Add(-duration)
	query := "SELECT COUNT(*) FROM login_failures WHERE zone_id = ? AND site_id = ? AND date BETWEEN ? AND ?"
	err := database.DB.QueryRow(query, zoneID, siteID, startTime, endTime).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// InsertLoginFailure 插入登陆失败的记录
func InsertLoginFailure(zoneID string, siteID string, date time.Time, deviceID string) error {
	insertQuery := "INSERT INTO login_failures (zone_id, site_id, date, device_id) VALUES (?, ?, ?, ?)"
	if _, err := database.DB.Exec(insertQuery, zoneID, siteID, date, deviceID); err != nil {
		return err
	}
	return nil
}
