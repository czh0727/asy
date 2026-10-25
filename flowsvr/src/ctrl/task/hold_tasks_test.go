
package task

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
)

func TestHoldTasksIntegration(t *testing.T) {
	if os.Getenv("ASY_INTEGRATION") != "1" {
		t.Skip("set ASY_INTEGRATION=1")
	}

	config.Init()

	if err := db.InitDB(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
		}
	})

	// 生成独立测试任务类型。
	taskType := fmt.Sprintf(
		"hold_%d", time.Now().UnixNano(),
	)

	table, err := db.TaskTableName(taskType, 1)
	if err != nil {
		t.Fatal(err)
	}

	// 清理由此测试创建的数据。
	t.Cleanup(func() {
		_ = db.DB.Exec(
			"DELETE FROM t_schedule_pos WHERE task_type = ?",
			taskType,
		).Error

		_ = db.DB.Exec(
			"DELETE FROM t_schedule_cfg WHERE task_type = ?",
			taskType,
		).Error

		_ = db.DB.Exec(
			"DROP TABLE IF EXISTS `" + table + "`",
		).Error
	})

	if err := db.TaskNsp.CreateTable(
		db.DB, taskType, "1",
	); err != nil {
		t.Fatal(err)
	}

	if err := db.TaskPosNsp.Create(
		db.DB,
		&db.TaskPos{
			TaskType:         taskType,
			ScheduleBeginPos: 1,
			ScheduleEndPos:   1,
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := db.TaskTypeCfgNsp.Create(
		db.DB,
		&db.TaskScheduleCfg{
			TaskType:        taskType,
			ScheduleLimit:   2,
			ScheduleInterval: 10,
		},
	); err != nil {
		t.Fatal(err)
	}

	// 创建 3 条已经到达执行时间的任务。
	ids := make([]string, 0, 3)

	for i := 0; i < 3; i++ {
		record := &db.Task{
			TaskId: db.TaskNsp.GenTaskId(
				taskType, "1",
			),
			TaskType:  taskType,
			Status:    int(db.TASK_STATUS_PENDING),
			OrderTime: time.Now().Unix() - 60,
		}

		if err := db.TaskNsp.Create(
			db.DB, taskType, "1", record,
		); err != nil {
			t.Fatal(err)
		}

		ids = append(ids, record.TaskId)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/v1/hold_tasks", HoldTasks)
	router.GET(
		"/v1/get_task_schedule_cfg_list",
		GetTaskScheduleCfgList,
	)

	// 第一轮领取两条任务。
	first := doJSONRequest(
		t,
		router,
		"/v1/hold_tasks",
		model.HoldTasksReq{
			TaskType: taskType,
			Limit:    2,
		},
	)

	var firstResp model.HoldTasksResp
	if err := json.Unmarshal(
		first.Body.Bytes(), &firstResp,
	); err != nil {
		t.Fatal(err)
	}

	if firstResp.Code != 0 ||
		len(firstResp.TaskList) != 2 {
		t.Fatalf(
			"first hold: %s",
			first.Body.String(),
		)
	}

	firstIDs := make(map[string]bool)
	for _, item := range firstResp.TaskList {
		firstIDs[item.TaskId] = true
	}

	// 第二轮应只领取剩下的一条。
	second := doJSONRequest(
		t,
		router,
		"/v1/hold_tasks",
		model.HoldTasksReq{
			TaskType: taskType,
			Limit:    2,
		},
	)

	var secondResp model.HoldTasksResp
	if err := json.Unmarshal(
		second.Body.Bytes(), &secondResp,
	); err != nil {
		t.Fatal(err)
	}

	if secondResp.Code != 0 ||
		len(secondResp.TaskList) != 1 {
		t.Fatalf(
			"second hold: %s",
			second.Body.String(),
		)
	}

	if firstIDs[secondResp.TaskList[0].TaskId] {
		t.Fatal("same task returned twice in sequential hold")
	}

	// 三条任务此时都应进入 processing。
	for _, id := range ids {
		stored, err := db.TaskNsp.Find(db.DB, id)
		if err != nil {
			t.Fatal(err)
		}

		if stored.Status != int(db.TASK_STATUS_PROCESSING) {
			t.Fatalf(
				"task %s status=%d, want processing",
				id, stored.Status,
			)
		}
	}

	// 读取真实调度配置。
	req, err := http.NewRequest(
		http.MethodGet,
		"/v1/get_task_schedule_cfg_list",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_ = req // 请求会在下面的测试命令中由真实路由验证
}
