package config

import (
	"log"
	"os"
)

var (
	PREDICTPORT string // 预测服务端口

	K8SNAMSPACE string // K8S命名空间

	MYSQLHOST     string // MYSQL服务地址
	MYSQLPORT     string // MYSQL服务端口
	MYSQLUSER     string // MYSQL服务用户
	MYSQLPASSWORD string // MYSQL服务密码
	MYSQLDATABASE string // MYSQL服务数据库

	MANAGERPROTOCOL = "http" // 资源管理模块服务协议
	MANAGERHOST     string   // 资源管理模块服务地址
	MANAGERPORT     string   // 资源管理模块服务端口

	TIMESNETPROTOCOL = "http" // 算法服务协议
	TIMESNETHOST     string   // 算法服务地址
	TIMESNETPORT     string   // 算法服务端口
)

func init() {
	PREDICTPORT = os.Getenv("PREDICT_PORT")
	if PREDICTPORT == "" {
		log.Fatalf("Failed to get predict port from env")
	}

	K8SNAMSPACE = os.Getenv("NAMESPACE")
	if K8SNAMSPACE == "" {
		log.Fatalf("Failed to get namespace from env")
	}

	MANAGERHOST = os.Getenv("MANAGER_HOST")
	if MANAGERHOST == "" {
		log.Fatalf("Failed to get manager host from env")
	}

	MANAGERPORT = os.Getenv("MANAGER_PORT")
	if MANAGERPORT == "" {
		log.Fatalf("Failed to get manager port from env")
	}

	MYSQLHOST = os.Getenv("MYSQL_HOST")
	if MYSQLHOST == "" {
		log.Fatalf("Failed to get mysql host from env")
	}

	MYSQLPORT = os.Getenv("MYSQL_PORT")
	if MYSQLPORT == "" {
		log.Fatalf("Failed to get mysql port from env")
	}

	MYSQLUSER = os.Getenv("MYSQL_USER")
	if MYSQLUSER == "" {
		log.Fatalf("Failed to get mysql user from env")
	}

	MYSQLPASSWORD = os.Getenv("MYSQL_PASSWORD")
	if MYSQLPASSWORD == "" {
		log.Fatalf("Failed to get mysql password from env")
	}

	MYSQLDATABASE = os.Getenv("MYSQL_DATABASE")
	if MYSQLDATABASE == "" {
		log.Fatalf("Failed to get mysql database from env")
	}

	TIMESNETHOST = os.Getenv("TIMESNET_HOST")
	if TIMESNETHOST == "" {
		log.Fatalf("Failed to get timesnet host from env")
	}

	TIMESNETPORT = os.Getenv("TIMESNET_PORT")
	if TIMESNETPORT == "" {
		log.Fatalf("Failed to get timesnet port from env")
	}
}