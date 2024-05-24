package config

import (
	"log"
	"os"
)

var (
	MANAGERPORT string // 资源管理模块服务端口

	K8SNAMSPACE   string // K8S命名空间
	K8SCONFIGPATH string // K8S配置文件地址

	MYSQLHOST     string // MYSQL服务地址
	MYSQLPORT     string // MYSQL服务端口
	MYSQLUSER     string // MYSQL服务用户
	MYSQLPASSWORD string // MYSQL服务密码
	MYSQLDATABASE string // MYSQL服务数据库
)

func init() {
	K8SNAMSPACE = os.Getenv("NAMESPACE")
	if K8SNAMSPACE == "" {
		log.Fatalf("Failed to get namespace from env")
	}

	K8SCONFIGPATH = os.Getenv("KUBECONFIG_PATH")
	if K8SCONFIGPATH == "" {
		log.Fatalf("Failed to get config path from env")
	}

	MANAGERPORT = os.Getenv("MANAGER_PORT")
	if MANAGERPORT == "" {
		log.Fatalf("Failed to get port from env")
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
}
