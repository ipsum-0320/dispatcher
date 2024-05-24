package mysql

import (
	"database/sql"
	"fmt"
	"predict/config"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func init() {
	//构建连接："用户名:密码@tcp(IP:端口)/数据库?charset=utf8"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", config.MYSQLUSER, config.MYSQLPASSWORD, config.MYSQLHOST, config.MYSQLPORT, config.MYSQLDATABASE)
	//打开数据库,前者是驱动名，所以要导入： _ "github.com/go-sql-driver/mysql"
	DB, _ = sql.Open("mysql", dsn)
	DB.SetConnMaxLifetime(100)
	DB.SetMaxIdleConns(1000)
	DB.SetMaxOpenConns(2000)
	//验证连接
	if err := DB.Ping(); err != nil {
		fmt.Println("open database fail")
		return
	}
	fmt.Println("connect success")
}
