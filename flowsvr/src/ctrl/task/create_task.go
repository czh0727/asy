package task

import (
	"fmt"
	"net/http"
	"time"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/constant"
	"github.com/czh0727/asy/flowsvr/src/ctrl/ctrlmodel"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
	"github.com/niuniumart/gosdk/handler"
)

type CreateTaskHandler struct {
	Req    model.CreateTaskReq
	Resp   model.CreateTaskResp
	UserId string
}

func CreateTask(c *gin.Context) {
	var hd CreateTaskHandler

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

// HandleInput 参数检查。
func (p *CreateTaskHandler) HandleInput() error {
	input := &p.Req.TaskData

	if err := db.ValidateTaskType(
		input.TaskType,
	); err != nil {
		p.Resp.Code = constant.ERR_INPUT_INVALID
		return err
	}

	if input.Priority != nil {
		if *input.Priority < 0 ||
			*input.Priority > db.MAX_PRIORITY {
			p.Resp.Code = constant.ERR_INPUT_INVALID
			return fmt.Errorf(
				"invalid task priority: %d",
				*input.Priority,
			)
		}
	}

	return nil
}

// HandleProcess 创建业务任务。
func (p *CreateTaskHandler) HandleProcess() error {
	input := &p.Req.TaskData

	// 1. 获取任务创建元数据。
	meta, err := cache.GetTaskCreateMeta(
		db.DB,
		input.TaskType,
	)
	if err != nil {
		p.Resp.Code = constant.ERR_GET_TASK_POS
		return err
	}

	// 2. 获取当前写入分表位置。
	if meta.EndPos <= 0 {
		return fmt.Errorf("invalid task write position")
	}

	scheduleEndPos := fmt.Sprintf("%d", meta.EndPos)

	// 3. 继承任务类型的重试配置。
	input.MaxRetryNum = meta.MaxRetryNum
	input.MaxRetryInterval = meta.RetryInterval

	// 4. 计算任务排序时间。
	input.OrderTime = calcInitialOrderTime(
		input.ProcessAt,
		input.Priority,
	)

	// 5. 转换为 Gorm 任务实体，生成 TaskId。
	var record db.Task

	if err := ctrlmodel.FillTaskModel(
		input, &record, scheduleEndPos,
	); err != nil {
		p.Resp.Code = constant.ERR_CREATE_TASK
		return err
	}

	// 6. 写入当前分表。
	err = db.TaskNsp.Create(
		db.DB,
		input.TaskType,
		scheduleEndPos,
		&record,
	)
	if err != nil {
		p.Resp.Code = constant.ERR_CREATE_TASK
		return err
	}

	// 7. 返回任务 ID。
	p.Resp.TaskId = record.TaskId

	// 8. 写 Redis 缓存。
	// 保留原项目语义：缓存失败不回滚 MySQL。
	if err := cache.CreateTask(&record); err != nil {
		return nil
	}

	return nil
}

// calcInitialOrderTime 计算初始调度排序时间。
//
// 未来执行的任务优先遵守 processAt；
// 立即执行的任务使用 now - priority。
func calcInitialOrderTime(
	processAt int64,
	priority *int,
) int64 {
	now := nowUnix()

	if processAt > now {
		return processAt
	}

	orderTime := now

	if priority != nil {
		orderTime -= int64(*priority)
	}

	return orderTime
}

// 保留原项目可替换时钟的测试入口。
var nowUnix = func() int64 {
	return timeNow().Unix()
}

var timeNow = func() time.Time {
	return time.Now()
}
