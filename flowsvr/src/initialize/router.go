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

	v1.POST("/hold_tasks", taskctrl.HoldTasks)

	v1.GET(
		"/get_task_schedule_cfg_list",
		taskctrl.GetTaskScheduleCfgList,
	)

	v1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
}
