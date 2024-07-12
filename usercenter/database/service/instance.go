package service

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"usercenter/database"
	"usercenter/database/model"
)

var loginMutex sync.Mutex

// 获取可用实例并接入终端
func GetInstanceAndLogin(zoneID string, siteID string, deviceID string) (*model.Instance, error) {

	loginMutex.Lock()
	defer loginMutex.Unlock()

	instance := &model.Instance{ZoneID: zoneID}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}

	var isElastic = false
	siteQuery := fmt.Sprintf(`SELECT * FROM instance_%s WHERE site_id = ? AND is_elastic = 0 AND status = 'available' LIMIT 1`, zoneID)
	// 先查询边缘是否有可用实例
	err = tx.QueryRow(siteQuery, siteID).Scan(&instance.SiteID, &instance.ServerIP, &instance.InstanceID, &instance.PodName, &instance.Port, &instance.IsElastic, &instance.Status, &instance.DeviceId)
	if err == sql.ErrNoRows { // 如果边缘没有的可用实例，再获取中心的可用实例
		isElastic = true
		centerQuery := fmt.Sprintf(`SELECT * FROM instance_%s WHERE is_elastic = 1 AND status = 'available' LIMIT 1`, zoneID)
		err = tx.QueryRow(centerQuery).Scan(&instance.SiteID, &instance.ServerIP, &instance.InstanceID, &instance.PodName, &instance.Port, &instance.IsElastic, &instance.Status, &instance.DeviceId)
	}

	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no available instance found for device %s", deviceID)
		} else {
			return nil, fmt.Errorf("error quering available instances: %w", err)
		}
	}

	var updateStmt string
	if isElastic { // 如果是弹性实例，则需要修改site_id
		updateStmt = fmt.Sprintf(`UPDATE instance_%s SET site_id = ?, status = "using", device_id = ? WHERE instance_id = ?`, instance.ZoneID)
		_, err = tx.Exec(updateStmt, instance.SiteID, deviceID, instance.InstanceID)
	} else {
		updateStmt = fmt.Sprintf(`UPDATE instance_%s SET status = "using", device_id = ? WHERE instance_id = ?`, instance.ZoneID)
		_, err = tx.Exec(updateStmt, deviceID, instance.InstanceID)
	}

	if err != nil {
		return nil, fmt.Errorf("error updating instance status: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("error commiting transaction: %w", err)
	}

	instance.Status = "using"
	instance.DeviceId = deviceID

	return instance, nil
}

// 根据终端id更新实例信息，登出设备
func LogoutDevice(ZoneID string, deviceID string) error {
	isElastic := -1 // 初始值，避免未使用的错误

	err := database.DB.QueryRow(fmt.Sprintf(`SELECT is_elastic FROM instance_%s WHERE device_id = ? LIMIT 1`, ZoneID), deviceID).Scan(&isElastic)
	if err != nil {
		return fmt.Errorf("%s cannot be found in %s table: %v", deviceID, ZoneID, err)
	}

	var updateStmt string
	if isElastic == 1 { // 如果是弹性实例就需要修改site_id为null
		updateStmt = fmt.Sprintf(`UPDATE instance_%s SET site_id = 'null', status = 'available', device_id = 'null' WHERE device_id = ?`, ZoneID)
	} else { // 否则就不需要
		updateStmt = fmt.Sprintf(`UPDATE instance_%s SET status = 'available', device_id = 'null' WHERE device_id = ?`, ZoneID)
	}

	_, err = database.DB.Exec(updateStmt, deviceID)
	if err != nil {
		return fmt.Errorf("failed to update instance information when %s logged out from %s: %v", deviceID, ZoneID, err)
	}
	return nil
}

func GetZoneListInDB() (map[string][]string, error) {
	rows, err := database.DB.Query("SHOW TABLES LIKE 'instance_%'")
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
	rows, err := database.DB.Query(fmt.Sprintf("SELECT DISTINCT site_id FROM instance_%s", ZoneID))
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
