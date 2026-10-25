package task

import (
	"fmt"
	"net/http"

	"github.com/czh0727/asy/flowsvr/src/constant"
	"github.com/czh0727/asy/flowsvr/src/ctrl/ctrlmodel"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
	"github.com/niuniumart/gosdk/handler"
)

type HoldTasksHandler struct {
	Req    model.HoldTasksReq
	Resp   model.HoldTasksResp
	UserId string
}

func HoldTasks(c *gin.Context) {
	var hd HoldTasksHandler

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

func (p *HoldTasksHandler) HandleInput() error {
	if err := db.ValidateTaskType(
		p.Req.TaskType,
	); err != nil {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return err
	}

	if p.Req.Limit < 0 {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return fmt.Errorf("limit cannot be negative")
	}

	return nil
}

func (p *HoldTasksHandler) HandleProcess() error {
	// 1. 使用原项目的领取数量限制。
	limit := p.Req.Limit

	if limit == 0 {
		limit = constant.DEFAULT_TASK_LIST_LIMIT
	}
	if limit > constant.MAX_TASK_LIST_LIMIT {
		limit = constant.MAX_TASK_LIST_LIMIT
	}

	// 2. 获取任务类型的当前调度读取位置。
	taskPos, err := db.TaskPosNsp.GetTaskPos(
		db.DB,
		p.Req.TaskType,
	)
	if err != nil {
		p.Resp.Code =
			constant.ERR_GET_TASK_SET_POS_FROM_DB
		return err
	}

	pos := fmt.Sprintf(
		"%d", taskPos.ScheduleBeginPos,
	)

	// 3. 查询当前分表里待执行的任务。
	taskList, err := db.TaskNsp.GetTaskList(
		db.DB,
		pos,
		p.Req.TaskType,
		db.TASK_STATUS_PENDING,
		limit,
	)
	if err != nil {
		p.Resp.Code =
			constant.ERR_GET_TASK_LIST_FROM_DB
		return err
	}

	ids := make([]string, 0, len(taskList))
	p.Resp.TaskList = make(
		[]*model.TaskData,
		0,
		len(taskList),
	)

	// 4. 数据库任务转换为 RPC 模型。
	for _, record := range taskList {
		ids = append(ids, record.TaskId)

		data := new(model.TaskData)
		ctrlmodel.FillTaskResp(record, data)

		p.Resp.TaskList = append(
			p.Resp.TaskList, data,
		)
	}

	// 5. 批量将任务标记为 processing。
	if len(ids) > 0 {
		if err := db.TaskNsp.BatchSetStatus(
			db.DB,
			ids,
			db.TASK_STATUS_PROCESSING,
		); err != nil {
			p.Resp.Code = constant.ERR_SET_TASK
			p.Resp.TaskList = nil
			return err
		}
	}

	return nil
}
