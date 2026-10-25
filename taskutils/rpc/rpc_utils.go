
package rpc

import (
	"net/http"
	"time"
)

var TaskClient *http.Client

func init() {
	InitClient()
}

// InitClient 初始化 HTTP 连接池。
func InitClient() {
	tr := &http.Transport{
		MaxIdleConns:        600,
		MaxIdleConnsPerHost: 600,
		IdleConnTimeout:     10 * time.Second,
	}

	TaskClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}
}

// GetTodayTime 获取本地当天零点的 Unix 时间戳。
func GetTodayTime() int64 {
	now := time.Now()

	return time.Date(
		now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0,
		now.Location(),
	).Unix()
}
