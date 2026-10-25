package task

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
)

func TestSetTaskAndGetTaskIntegration(t *testing.T) {
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

	if err := cache.InitCache(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cache.Close()
	})

	// 使用独立任务类型，不影响原来的 lark 数据。
	taskType := fmt.Sprintf(
		"stage_%d", time.Now().UnixNano(),
	)

	table, err := db.TaskTableName(taskType, 1)
	if err != nil {
		t.Fatal(err)
	}

	var createdID string

	t.Cleanup(func() {
		if createdID != "" {
			cache.OnceDeleteTask(createdID)
		}

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

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/v1/register_task", RegisterTask)
	router.POST("/v1/create_task", CreateTask)
	router.POST("/v1/set_task", SetTask)
	router.GET("/v1/get_task", GetTask)

	// 第一步：注册新的任务类型。
	register := doJSONRequest(
		t,
		router,
		"/v1/register_task",
		model.RegisterTaskReq{
			TaskType: taskType,
		},
	)

	var registerResp model.RegisterTaskResp
	if err := json.Unmarshal(
		register.Body.Bytes(),
		&registerResp,
	); err != nil {
		t.Fatal(err)
	}
	if registerResp.Code != 0 {
		t.Fatalf(
			"register failed: %s",
			register.Body.String(),
		)
	}

	// 第二步：创建任务。
	create := doJSONRequest(
		t,
		router,
		"/v1/create_task",
		model.CreateTaskReq{
			TaskData: model.TaskData{
				TaskType:    taskType,
				TaskStage:   "sendmsg",
				UserId:      "chapter6",
				TaskContext: `{"step":1}`,
			},
		},
	)

	var createResp model.CreateTaskResp
	if err := json.Unmarshal(
		create.Body.Bytes(),
		&createResp,
	); err != nil {
		t.Fatal(err)
	}

	if createResp.Code != 0 ||
		createResp.TaskId == "" {
		t.Fatalf(
			"create failed: %s",
			create.Body.String(),
		)
	}

	createdID = createResp.TaskId

	// 第三步：检查创建后确实进入 Redis 缓存。
	cached, err := cache.FindTaskByTaskId(createdID)
	if err != nil {
		t.Fatal(err)
	}

	if cached == nil ||
		cached.Status != int(db.TASK_STATUS_PENDING) {
		t.Fatalf("unexpected cache: %+v", cached)
	}

	// 第四步：模拟任务业务执行完成后的 SetTask。
	setResult := doJSONRequest(
		t,
		router,
		"/v1/set_task",
		model.SetTaskReq{
			TaskData: model.TaskData{
				TaskId:      createdID,
				TaskType:    taskType,
				TaskStage:   "record",
				Status:      int(db.TASK_STATUS_SUCCESS),
				UserId:      "chapter6",
				TaskContext: `{"step":2}`,
				ScheduleLog: `{
					"LastData":{
						"TraceId":"trace-001",
						"ErrMsg":"",
						"Cost":"120ms"
					}
				}`,
				OrderTime: time.Now().Unix(),
			},
		},
	)

	var setResp model.SetTaskResp
	if err := json.Unmarshal(
		setResult.Body.Bytes(),
		&setResp,
	); err != nil {
		t.Fatal(err)
	}

	if setResp.Code != 0 {
		t.Fatalf(
			"set task failed: %s",
			setResult.Body.String(),
		)
	}

	// 第五步：确认 MySQL 状态、阶段、上下文均已修改。
	stored, err := db.TaskNsp.Find(
		db.DB, createdID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != int(db.TASK_STATUS_SUCCESS) ||
		stored.TaskStage != "record" ||
		stored.TaskContext != `{"step":2}` {
		t.Fatalf("database task mismatch: %+v", stored)
	}

	// 第六步：确认旧缓存已被删除。
	cached, err = cache.FindTaskByTaskId(createdID)
	if err != nil {
		t.Fatal(err)
	}
	if cached != nil {
		t.Fatalf("cache should be deleted: %+v", cached)
	}

	// 第七步：GET /v1/get_task。
	// 应从数据库读取最新任务，并回填缓存。
	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/get_task?task_id="+createdID,
		nil,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"get task status=%d",
			recorder.Code,
		)
	}

	var getResp model.GetTaskResp
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&getResp,
	); err != nil {
		t.Fatal(err)
	}

	if getResp.Code != 0 ||
		getResp.TaskData == nil {
		t.Fatalf(
			"get task failed: %s",
			recorder.Body.String(),
		)
	}

	if getResp.TaskData.Status !=
		int(db.TASK_STATUS_SUCCESS) ||
		getResp.TaskData.TaskStage != "record" ||
		getResp.TaskData.TaskContext != `{"step":2}` {
		t.Fatalf(
			"get task returned stale data: %+v",
			getResp.TaskData,
		)
	}
}
