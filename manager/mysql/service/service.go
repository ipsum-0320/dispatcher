package service

import (
	"database/sql"
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

func GetAvailableInstanceInCenter(zoneId string) (int32, error) {
	rows, err := mysql.DB.Query(fmt.Sprintf("SELECT DISTINCT count(*) AS COUNT FROM instance_%s WHERE is_elastic = 1 AND status = 'available'", zoneId))
	if err != nil {
		fmt.Printf("%s: query current available instance failed, err: %v\n", zoneId, err)
		return 0, err
	}
	defer func(query *sql.Rows) {
		if err := query.Close(); err != nil {
			fmt.Printf("%s: close current available instance failed, err: %v\n", zoneId, err)
		}
	}(rows)

	var count int32
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			fmt.Printf("%s: scan current available instance failed, err: %v\n", zoneId, err)
			return 0, err
		}
	}
	return count, nil
}

func SynchronizeInstanceStatus(zoneId string, instanceName string, status string) error {
	statusInDB, err := getInstanceStatus(zoneId, instanceName)
	if err != nil {
		return err
	}

	if statusInDB != status {
		return updateInstanceStatus(zoneId, instanceName, status)
	}

	return nil
}

func getInstanceStatus(zoneId string, instanceName string) (string, error) {
	row := mysql.DB.QueryRow(fmt.Sprintf("SELECT status FROM instance_%s WHERE instance_id = ?", zoneId), instanceName)
	var status string
	err := row.Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("pod with name %s not found in the database : %w", instanceName, err)
		}
		return "", fmt.Errorf("error scanning row: %w", err)
	}
	return status, nil
}

func updateInstanceStatus(zoneId string, instanceName string, status string) error {
	result, err := mysql.DB.Exec(fmt.Sprintf("UPDATE instance_%s SET status = ? WHERE instance_id = ?", zoneId), status, instanceName)
	if err != nil {
		return fmt.Errorf("error executing update: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no rows were updated")
	}
	return nil
}
