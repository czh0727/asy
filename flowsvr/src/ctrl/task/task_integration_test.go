package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
)

func TestRegisterAndCreateTaskIntegration(t *testing.T) {
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

	// 使用唯一任务类型，不影响已有的 lark 数据。
	taskType := fmt.Sprintf(
		"ch4_%d",
		time.Now().UnixNano(),
	)

	table, err := db.TaskTableName(taskType, 1)
	if err != nil {
		t.Fatal(err)
	}

	var taskID string

	t.Cleanup(func() {
		if taskID != "" {
			cache.OnceDeleteTask(taskID)
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

	// 第一步：注册新的任务类型。
	registerBody := model.RegisterTaskReq{
		TaskType:      taskType,
		ScheduleLimit: 5,
		MaxRetryNum:   2,
		RetryInterval: 4,
	}

	registerResp := doJSONRequest(
		t, router, "/v1/register_task",
		registerBody,
	)

	var registration model.RegisterTaskResp
	if err := json.Unmarshal(
		registerResp.Body.Bytes(),
		&registration,
	); err != nil {
		t.Fatal(err)
	}

	if registration.Code != 0 {
		t.Fatalf(
			"register response: %s",
			registerResp.Body.String(),
		)
	}

	// 第二步：创建一条未来调度的业务任务。
	priority := 10
	processAt := time.Now().Unix() + 60

	createBody := model.CreateTaskReq{
		TaskData: model.TaskData{
			TaskType:    taskType,
			TaskStage:   "sendmsg",
			UserId:      "test-user",
			Priority:    &priority,
			TaskContext: `{"msg":"hello"}`,
			ProcessAt:   processAt,
		},
	}

	createResp := doJSONRequest(
		t, router, "/v1/create_task",
		createBody,
	)

	var created model.CreateTaskResp
	if err := json.Unmarshal(
		createResp.Body.Bytes(),
		&created,
	); err != nil {
		t.Fatal(err)
	}

	if created.Code != 0 || created.TaskId == "" {
		t.Fatalf(
			"create response: %s",
			createResp.Body.String(),
		)
	}

	taskID = created.TaskId

	// 第三步：通过 TaskId 定位 MySQL 任务表。
	stored, err := db.TaskNsp.Find(
		db.DB, taskID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != int(db.TASK_STATUS_PENDING) ||
		stored.MaxRetryNum != 2 ||
		stored.MaxRetryInterval != 4 ||
		stored.OrderTime != processAt {
		t.Fatalf(
			"unexpected task: %+v",
			stored,
		)
	}

	if !strings.HasSuffix(
		taskID, "_"+strings.ReplaceAll(taskType, "_", "-")+"_1",
	) {
		t.Fatalf("unexpected task ID: %s", taskID)
	}

	// 第四步：验证 Redis 缓存。
	cached, err := cache.FindTaskByTaskId(taskID)
	if err != nil {
		t.Fatal(err)
	}

	if cached == nil || cached.TaskId != taskID {
		t.Fatalf(
			"unexpected cached task: %+v",
			cached,
		)
	}

	// 第五步：检查分表位置。
	pos, err := db.TaskPosNsp.GetTaskPos(
		db.DB, taskType,
	)
	if err != nil {
		t.Fatal(err)
	}

	if pos.ScheduleBeginPos != 1 ||
		pos.ScheduleEndPos != 1 {
		t.Fatalf("unexpected position: %+v", pos)
	}
}

func doJSONRequest(
	t *testing.T,
	router http.Handler,
	path string,
	body interface{},
) *httptest.ResponseRecorder {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		path,
		bytes.NewReader(data),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf(
			"path=%s status=%d body=%s",
			path, w.Code, w.Body.String(),
		)
	}

	return w
}
