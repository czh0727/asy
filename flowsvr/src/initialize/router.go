package initialize

import (
	"net/http"

	taskctrl "github.com/czh0727/asy/flowsvr/src/ctrl/task"
	"github.com/gin-gonic/gin"
)

// RegisterRouter 注册 flowsvr HTTP 接口。
func RegisterRouter(router *gin.Engine) {
	v1 := router.Group("/v1")

	v1.POST("/register_task", taskctrl.RegisterTask)
	v1.POST("/create_task", taskctrl.CreateTask)

	v1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
}
