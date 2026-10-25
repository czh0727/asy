package task

import (
	"fmt"
	"net/http"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/constant"
	"github.com/czh0727/asy/flowsvr/src/ctrl/ctrlmodel"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
	"github.com/niuniumart/gosdk/handler"
)

type SetTaskHandler struct {
	Req    model.SetTaskReq
	Resp   model.SetTaskResp
	UserId string
}

func SetTask(c *gin.Context) {
	var hd SetTaskHandler

	defer func() {
		hd.Resp.Msg = constant.GetErrMsg(
			hd.Resp.Code,
		)
		c.JSON(http.StatusOK, hd.Resp)
	}()

	hd.UserId = c.Request.Header.Get(
		constant.HEADER_USERID,
	)

	if err := c.ShouldBind(&hd.Req); err != nil {
		hd.Resp.Code = constant.ERR_SHOULD_BIND
		return
	}

	_ = handler.Run(&hd)
}

// HandleInput 检查请求参数。
func (p *SetTaskHandler) HandleInput() error {
	data := &p.Req.TaskData

	if data.TaskId == "" {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return fmt.Errorf("task id is required")
	}

	taskType, _, err := db.ParseTaskID(data.TaskId)
	if err != nil || taskType != data.TaskType {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return fmt.Errorf(
			"task id and task type do not match",
		)
	}

	if !db.IsValidStatus(db.TaskEnum(data.Status)) {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return fmt.Errorf("invalid status: %d", data.Status)
	}

	if data.Priority != nil {
		if *data.Priority < 0 ||
			*data.Priority > db.MAX_PRIORITY {
			p.Resp.Code = constant.ERR_INPUT_INVALID
			return fmt.Errorf("invalid priority")
		}
	}

	return nil
}

// HandleProcess 持久化任务执行结果。
func (p *SetTaskHandler) HandleProcess() error {
	record := new(db.Task)

	err := ctrlmodel.FillTaskModel(
		&p.Req.TaskData,
		record,
		"",
	)
	if err != nil {
		p.Resp.Code = constant.ERR_SET_TASK
		return err
	}

	// 更新 MySQL。
	if err := record.UpdateTask(db.DB); err != nil {
		p.Resp.Code = constant.ERR_SET_TASK
		return err
	}

	p.Resp.Code = constant.SUCCESS

	// MySQL 更新成功后，让 Redis 的旧缓存失效。
	cache.OnceDeleteTask(record.TaskId)
	return nil
}
