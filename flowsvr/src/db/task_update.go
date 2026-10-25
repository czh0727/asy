package db

import (
	"errors"
	"time"

	"github.com/jinzhu/gorm"
)

// UpdateTask 将 Worker 的任务执行结果更新到 MySQL。
// 原项目只允许更新非终态任务。
func (p *Task) UpdateTask(database *gorm.DB) error {
	if p == nil {
		return errors.New("task is nil")
	}

	taskType, pos, err := ParseTaskID(p.TaskId)
	if err != nil {
		return err
	}

	// 避免请求中的 task_type 与 TaskId 中编码的类型不一致。
	// 这是重建时增加的输入校验。
	if p.TaskType != taskType {
		return errors.New("task type mismatch")
	}

	table, err := TaskTableName(taskType, pos)
	if err != nil {
		return err
	}

	p.ModifyTime = time.Now()

	return database.Table(table).
		Where("task_id = ?", p.TaskId).
		Where(
			"status <> ? AND status <> ?",
			TASK_STATUS_SUCCESS,
			TASK_STATUS_FAILED,
		).
		Updates(p).Error
}
