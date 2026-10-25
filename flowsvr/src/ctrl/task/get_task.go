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

type GetTaskHandler struct {
	Req    model.GetTaskReq
	Resp   model.GetTaskResp
	UserId string
}

func GetTask(c *gin.Context) {
	var hd GetTaskHandler

	defer func() {
		hd.Resp.Msg = constant.GetErrMsg(hd.Resp.Code)
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

func (p *GetTaskHandler) HandleInput() error {
	if _, _, err := db.ParseTaskID(
		p.Req.TaskId,
	); err != nil {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return fmt.Errorf("invalid task id: %w", err)
	}
	return nil
}

func (p *GetTaskHandler) HandleProcess() error {
	// 1. 优先读取 Redis 缓存。
	cached, err := cache.FindTaskByTaskId(
		p.Req.TaskId,
	)
	if err != nil {
		p.Resp.Code = constant.ERR_GET_TASK_INFO
		return err
	}

	if cached != nil {
		data := new(model.TaskData)
		ctrlmodel.FillTaskResp(cached, data)
		p.Resp.TaskData = data
		return nil
	}

	// 2. 未命中缓存，查询对应的 MySQL 分表。
	stored, err := db.TaskNsp.Find(
		db.DB,
		p.Req.TaskId,
	)
	if err != nil {
		p.Resp.Code = constant.ERR_GET_TASK_INFO
		return err
	}

	// 3. 尝试回填缓存。
	// 与原项目保持相同语义：缓存回填失败不影响查库结果。
	_ = cache.CreateTask(stored)

	data := new(model.TaskData)
	ctrlmodel.FillTaskResp(stored, data)
	p.Resp.TaskData = data

	return nil
}
