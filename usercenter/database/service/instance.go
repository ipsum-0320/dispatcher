package service

import (
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

	// 获取边缘可用的实例
	instance, err := getAvailableInstanceFromSite(zoneID, siteID)
	if err == nil {
		// 边缘有可用实例
		instance, err := loginDevice(instance, deviceID, "site")
		if err != nil {
			return nil, fmt.Errorf("failed to update instance information in %s: %v", siteID, err)
		}
		return instance, nil
	}

	// 获取中心可用实例
	instance, err = getAvailableInstanceFromCenter(zoneID)
	if err == nil {
		// 中心有可用实例
		instance.SiteID = siteID // 弹性实例需要额外给site_id赋值
		instance, err := loginDevice(instance, deviceID, "center")
		if err != nil {
			return nil, fmt.Errorf("failed to update instance information in %s: %v", zoneID, err)
		}
		return instance, nil
	}

	return nil, fmt.Errorf("no available instance to be found for %s: %v", deviceID, err)
}

// 根据终端id更新实例信息，登出设备
func LogoutDevice(zoneID string, deviceID string) error {
	isElastic := -1 // 初始值，避免未使用的错误

	err := database.DB.QueryRow(fmt.Sprintf(`SELECT is_elastic FROM instance_%s WHERE device_id = ? LIMIT 1`, zoneID), deviceID).Scan(&isElastic)
	if err != nil {
		return fmt.Errorf("%s cannot be found in %s table: %v", deviceID, zoneID, err)
	}

	var updateStmt string
	if isElastic == 1 { // 如果是弹性实例就需要修改site_id为null
		updateStmt = fmt.Sprintf(`UPDATE instance_%s SET site_id = 'null', status = 'available', device_id = 'null' WHERE device_id = ?`, zoneID)
	} else { // 否则就不需要
		updateStmt = fmt.Sprintf(`UPDATE instance_%s SET status = 'available', device_id = 'null' WHERE device_id = ?`, zoneID)
	}

	_, err = database.DB.Exec(updateStmt, deviceID)
	if err != nil {
		return fmt.Errorf("failed to update instance information when %s logged out from %s: %v", deviceID, zoneID, err)
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
		zoneID := strings.TrimPrefix(tableName, "instance_")

		sites, err := GetSiteListInZone(zoneID)
		if err != nil {
			log.Printf("Error getting unique site IDs for %s: %v", tableName, err)
			continue
		}
		zoneList[zoneID] = sites
	}

	fmt.Println(zoneList)
	return zoneList, nil
}

func GetSiteListInZone(zoneID string) ([]string, error) {
	rows, err := database.DB.Query(fmt.Sprintf("SELECT DISTINCT site_id FROM instance_%s", zoneID))
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

func getAvailableInstanceFromSite(zoneID string, siteID string) (*model.Instance, error) {
	instance := &model.Instance{ZoneID: zoneID}

	query := `SELECT * FROM instance_%s WHERE site_id = ? AND is_elastic = 0 AND status = 'available' LIMIT 1`
	stmt, err := database.DB.Prepare(fmt.Sprintf(query, zoneID))
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	err = stmt.QueryRow(siteID).Scan(&instance.SiteID, &instance.ServerIP, &instance.InstanceID, &instance.PodName, &instance.Port, &instance.IsElastic, &instance.Status, &instance.DeviceId)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func getAvailableInstanceFromCenter(zoneID string) (*model.Instance, error) {
	instance := &model.Instance{ZoneID: zoneID}

	query := `SELECT * FROM instance_%s WHERE is_elastic = 1 AND status = 'available' LIMIT 1`
	stmt, err := database.DB.Prepare(fmt.Sprintf(query, zoneID))
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow().Scan(&instance.SiteID, &instance.ServerIP, &instance.InstanceID, &instance.PodName, &instance.Port, &instance.IsElastic, &instance.Status, &instance.DeviceId)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

// loginDevice 登出时更新实例的状态和设备ID
func loginDevice(instance *model.Instance, deviceID string, position string) (*model.Instance, error) {
	var err error
	if position == "center" {
		updateStmt := fmt.Sprintf(`UPDATE instance_%s SET site_id = ?, status = "using", device_id = ? WHERE instance_id = ?`, instance.ZoneID)
		_, err = database.DB.Exec(updateStmt, instance.SiteID, deviceID, instance.InstanceID)
	} else if position == "site" {
		updateStmt := fmt.Sprintf(`UPDATE instance_%s SET status = "using", device_id = ? WHERE instance_id = ?`, instance.ZoneID)
		_, err = database.DB.Exec(updateStmt, deviceID, instance.InstanceID)
	}
	if err != nil {
		return nil, err
	}

	instance.Status = "using"
	instance.DeviceId = deviceID

	return instance, nil
}
