package main

import (
	"database/sql"
	"fmt"
	"predict/manager"
	"predict/mysql"
	"predict/timesnet"
)

func Process() error {
	rows, err := mysql.DB.Query("")
	if err != nil {
		fmt.Printf("query failed, err:%v\n", err)
		return err
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("close query failed, err:%v\n", err)
		}
	}(rows)
	/*
		实例数表：
		+----+--------+-----------+------+---------+------+-------+------------------+
		| id | Site_id   | Date  | instances  |
		+----+--------+-----------+------+---------+------+-------+------------------+
		|  1 | hangzhou | 2024-03-26 12:00:00 |  1000     |
		|  2 | hangzhou | 2024-03-26 12:01:00 |  2000     |
		|  3 | ningbo   | 2024-03-26 12:00:00 |  3000     |
		|  4 | ningbo   | 2024-03-26 12:01:00 |  4000     |
		+----+--------+-----------+------+---------+------+-------+------------------+
	*/
	predMap := make(timesnet.PredDataSource)
	for rows.Next() {
		var (
			id        int64
			siteId    string
			date      string
			instances int32
		)
		if err := rows.Scan(&id, &siteId, &date, &instances); err != nil {
			fmt.Printf("scan failed: %v\n", err)
			return err
		}
		predMap[date] = instances
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("error during iteration: %v\n", err)
		return err
	}

	predResponse, err := timesnet.Predict(predMap)
	if err != nil {
		fmt.Printf("predict failed, err:%v\n", err)
		return err
	}

	// TODO
	manager.Manage(predResponse)
	return nil
}
