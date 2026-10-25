package ctrlmodel

import (
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
)

// FillTaskModel 把请求结构转换为数据库实体。
func FillTaskModel(
	source *model.TaskData,
	target *db.Task,
	scheduleEndPos string,
) error {
	if source.TaskId == "" {
		target.TaskId = db.TaskNsp.GenTaskId(
			source.TaskType,
			scheduleEndPos,
		)
		target.Status = int(db.TASK_STATUS_PENDING)
	} else {
		target.TaskId = source.TaskId
		target.Status = source.Status
	}

	target.UserId = source.UserId
	target.TaskType = source.TaskType
	target.TaskStage = source.TaskStage
	target.ScheduleLog = source.ScheduleLog
	target.CrtRetryNum = source.CrtRetryNum
	target.MaxRetryNum = source.MaxRetryNum
	target.MaxRetryInterval = source.MaxRetryInterval
	target.TaskContext = source.TaskContext
	target.OrderTime = source.OrderTime

	if source.Priority != nil {
		target.Priority = *source.Priority
	}

	return nil
}

// FillTaskResp 把数据库实体转换为 RPC TaskData。
func FillTaskResp(
	source *db.Task,
	target *model.TaskData,
) {
	target.UserId = source.UserId
	target.TaskId = source.TaskId
	target.TaskType = source.TaskType
	target.TaskStage = source.TaskStage
	target.Status = source.Status
	target.ScheduleLog = source.ScheduleLog

	priority := source.Priority
	target.Priority = &priority

	target.CrtRetryNum = source.CrtRetryNum
	target.MaxRetryNum = source.MaxRetryNum
	target.MaxRetryInterval = source.MaxRetryInterval
	target.TaskContext = source.TaskContext
	target.OrderTime = source.OrderTime
	target.CreateTime = source.CreateTime
	target.ModifyTime = source.ModifyTime
}
