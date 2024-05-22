package service

import (
	"fmt"
	"log"
	"manager/mysql"
)

func InsertInstance(zoneId string, siteId string, serverIp string, instanceId string, podName string, port int, is_elastic int, status string, device_id string) error {
	query := fmt.Sprintf("INSERT INTO instance_%s (site_id, server_ip, instance_id, pod_name, port, is_elastic, status, device_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", zoneId)
	stmt, err := mysql.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(siteId, serverIp, instanceId, podName, port, is_elastic, status, device_id)
	if err != nil {
		return err
	}
	return nil
}

func GetAndDeleteAvailableInstancesInCenter(zoneId string, num int32) ([]string, error) {
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT pod_name FROM instance_%s WHERE is_elastic = 1 AND status = 'available' ORDER BY RAND() LIMIT %d", zoneId, num))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var podList []string

	for rows.Next() {
		var podName string
		if err = rows.Scan(&podName); err != nil {
			return nil, err
		}
		podList = append(podList, podName)

		if _, err = mysql.DB.Exec(fmt.Sprintf("DELETE FROM instance_%s WHERE pod_name = ?", zoneId), podName); err != nil {
			log.Printf("Failed to delete instance %s from database", podName)
			return nil, err
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return podList, nil
}
