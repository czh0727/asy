package task

import (
	"net/http"

	"github.com/czh0727/asy/flowsvr/src/constant"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
	"github.com/niuniumart/gosdk/handler"
)

type RegisterTasksHandler struct {
	Req  model.RegisterTaskReq
	Resp model.RegisterTaskResp
}

func RegisterTask(c *gin.Context) {
	var hd RegisterTasksHandler

	defer func() {
		hd.Resp.Msg = constant.GetErrMsg(
			hd.Resp.Code,
		)
		c.JSON(http.StatusOK, hd.Resp)
	}()

	if err := c.ShouldBind(&hd.Req); err != nil {
		hd.Resp.Code = constant.ERR_SHOULD_BIND
		return
	}

	_ = handler.Run(&hd)
}

func (p *RegisterTasksHandler) HandleInput() error {
	if err := db.ValidateTaskType(
		p.Req.TaskType,
	); err != nil {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return err
	}
	return nil
}

func (p *RegisterTasksHandler) HandleProcess() error {
	taskType := p.Req.TaskType

	// Step 1：创建第一张任务表。
	if err := db.TaskNsp.CreateTable(
		db.DB, taskType, "1",
	); err != nil {
		p.Resp.Code =
			constant.ERR_REGISTER_TASK_CREATE_NEW_TABLE
		return err
	}

	// Step 2：初始化分表读写位置。
	err := db.TaskPosNsp.Create(
		db.DB,
		&db.TaskPos{
			TaskType:         taskType,
			ScheduleBeginPos: 1,
			ScheduleEndPos:   1,
		},
	)
	if err != nil {
		p.Resp.Code =
			constant.ERR_REGISTER_TASK_INSERT_RECORD
		return err
	}

	// Step 3：应用原项目的默认调度配置。
	scheduleLimit := p.Req.ScheduleLimit
	if scheduleLimit <= 0 {
		scheduleLimit = 100
	}

	scheduleInterval := p.Req.ScheduleInterval
	if scheduleInterval <= 0 {
		scheduleInterval = 1
	}

	maxProcessingTime := p.Req.MaxProcessingTime
	if maxProcessingTime <= 0 {
		maxProcessingTime = 300
	}

	maxRetryNum := p.Req.MaxRetryNum
	if maxRetryNum <= 0 {
		maxRetryNum = 3
	}

	retryInterval := p.Req.RetryInterval
	if retryInterval <= 0 {
		retryInterval = 10
	}

	maxRetryInterval := p.Req.MaxRetryInterval
	if maxRetryInterval <= 0 {
		maxRetryInterval = 300
	}

	cfg := &db.TaskScheduleCfg{
		TaskType:          taskType,
		ScheduleLimit:     scheduleLimit,
		ScheduleInterval:  scheduleInterval,
		MaxProcessingTime: maxProcessingTime,
		MaxRetryNum:       maxRetryNum,
		RetryInterval:     retryInterval,
		MaxRetryInterval:  maxRetryInterval,
	}

	if err := db.TaskTypeCfgNsp.Create(
		db.DB, cfg,
	); err != nil {
		p.Resp.Code =
			constant.ERR_REGISTER_TASK_INSERT_RECORD
		return err
	}

	return nil
}
