package config

import (
	"log"
	"os"
	"strconv"
)

var (
	MANAGERPORT = "6666" // 资源管理模块服务端口

	K8SNAMSPACE   string // K8S命名空间
	K8SCONFIGPATH string // K8S配置文件地址
	MYSQLHOST     string // MYSQL服务地址
	MYSQLPORT     string // MYSQL服务端口
	MYSQLUSER     string // MYSQL服务用户
	MYSQLPASSWORD string // MYSQL服务密码
	MYSQLDATABASE string // MYSQL服务数据库
	SCALERATIO    int    // 缩放比例
	HUADONGTOTAL  int    // 华东实例总数

	CENTERMAXTOTAL = 240   // 弹性实例数量上限
	CHECKINGSTATUS = false // 是否检查实例状态并同步到数据库
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

	MYSQLHOST = os.Getenv("MYSQL_SERVICE_SERVICE_HOST")
	if MYSQLHOST == "" {
		log.Fatalf("Failed to get mysql host from env")
	}

	MYSQLPORT = os.Getenv("MYSQL_SERVICE_SERVICE_PORT")
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

	var err error
	SCALERATIO, err = strconv.Atoi(os.Getenv("SCALE_RATIO"))
	if err != nil {
		log.Fatal("Failed to get instance scale ratio from env")
	} else if SCALERATIO == 0 {
		log.Fatal("Instance scale ratio cannot be zero")
	}

	HUADONGTOTAL, err = strconv.Atoi(os.Getenv("HUADONG_TOTAL"))
	if err != nil {
		log.Fatal("Failed to get number of total instances in huadong from env")
	} else if HUADONGTOTAL == 0 {
		log.Fatal("Number of total instances in huadong cannot be zero")
	}
}
