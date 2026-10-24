
package db

import (
	"fmt"
	"strings"

	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/jinzhu/gorm"
	"github.com/niuniumart/gosdk/gormcli"
)

// DB 保存 flowsvr 使用的 Gorm 数据库连接。
var DB *gorm.DB

// InitDB 初始化 MySQL。
func InitDB() error {
	if config.Conf == nil {
		return fmt.Errorf("config is not initialized")
	}

	// 使用原项目 gosdk 中的 Gorm 工厂。
	gormcli.Factory = gormcli.GormFactory{
		MaxIdleConn: config.Conf.MySQL.MaxIdleConn,
		MaxConn:     config.Conf.MySQL.MaxConn,
		IdleTimeout: config.Conf.MySQL.IdleTimeout,
	}

	database, err := gormcli.Factory.CreateGorm(
		config.Conf.MySQL.User,
		config.Conf.MySQL.Pwd,
		config.Conf.MySQL.Url,
		config.Conf.MySQL.Dbname,
	)
	if err != nil {
		return fmt.Errorf("create gorm connection: %w", err)
	}

	// sql.Open / Gorm 初始化成功不代表数据库真正可用。
	// 必须 Ping，确认网络和账号认证没有问题。
	if err := database.DB().Ping(); err != nil {
		_ = database.Close()
		return fmt.Errorf("ping mysql: %w", err)
	}

	DB = database

	fmt.Println("Database connection is available")
	return nil
}

const GORM_DUPLICATE_ERR_KEY = "Duplicate entry"

// IsDupErr 判断是否属于 MySQL 重复键错误。
func IsDupErr(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(
		err.Error(),
		GORM_DUPLICATE_ERR_KEY,
	)
}

// GetTaskTableName 保留原项目辅助函数。
// 实际滚动任务表还需要额外拼接 position。
func GetTaskTableName(taskType string) string {
	return fmt.Sprintf("t_%s_task", taskType)
}
