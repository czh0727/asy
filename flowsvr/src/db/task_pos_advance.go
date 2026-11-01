package db

import (
	"errors"
	"time"

	"github.com/jinzhu/gorm"
)

// AdvanceScheduleEndPos 推进写入位置。
//
// 使用 CAS：只有当前 end_pos 仍然等于 current，
// 才允许将其更新为 current+1。
func (p *TaskPos) AdvanceScheduleEndPos(
	database *gorm.DB,
	taskType string,
	current int,
) (bool, error) {

	if database == nil {
		return false, errors.New("database is nil")
	}

	if err := ValidateTaskType(taskType); err != nil {
		return false, err
	}

	if current <= 0 {
		return false, errors.New("invalid current position")
	}

	result := database.Table(p.TableName()).
		Where(
			"task_type = ? AND schedule_end_pos = ?",
			taskType,
			current,
		).
		UpdateColumns(map[string]interface{}{
			"schedule_end_pos": current + 1,
			"modify_time":      time.Now(),
		})

	return result.RowsAffected == 1, result.Error
}

// AdvanceScheduleBeginPos 推进调度读取位置。
//
// 只有当前表全部完成，而且距离最近一次分表操作
// 已经过30秒，才允许推进。
func (p *TaskPos) AdvanceScheduleBeginPos(
	database *gorm.DB,
	taskType string,
	current int,
) (bool, error) {

	if database == nil {
		return false, errors.New("database is nil")
	}

	if err := ValidateTaskType(taskType); err != nil {
		return false, err
	}

	if current <= 0 {
		return false, errors.New("invalid current position")
	}

	result := database.Table(p.TableName()).
		Where(
			"task_type = ? AND schedule_begin_pos = ? AND schedule_end_pos > ?",
			taskType,
			current,
			current,
		).
		Where(
			"modify_time < ?",
			time.Now().Add(-30*time.Second),
		).
		UpdateColumn(
			"schedule_begin_pos",
			current+1,
		)

	return result.RowsAffected == 1, result.Error
}
