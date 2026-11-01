package db

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
)

// GetLongTimeProcessing 获取超过最大执行时间的任务。
//
// 查询条件：
// 1. task_type 匹配
// 2. status = processing
// 3. modify_time 超过最大执行时间
func (p *Task) GetLongTimeProcessing(
	database *gorm.DB,
	taskType string,
	pos string,
	maxProcessTime int64,
	limit int,
) ([]*Task, error) {

	if database == nil {
		return nil, errors.New("database is nil")
	}

	if maxProcessTime <= 0 {
		return nil, errors.New("max process time must be positive")
	}

	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}

	position, err := strconv.Atoi(pos)
	if err != nil {
		return nil, err
	}

	table, err := TaskTableName(taskType, position)
	if err != nil {
		return nil, err
	}

	taskList := make([]*Task, 0)

	deadline := time.Now().Unix() - maxProcessTime

	err = database.Table(table).
		Where("task_type = ?", taskType).
		Where("status = ?", TASK_STATUS_PROCESSING).
		Where(
			"modify_time < FROM_UNIXTIME(?)",
			deadline,
		).
		Limit(limit).
		Find(&taskList).Error

	if err != nil {
		return nil, fmt.Errorf(
			"get long time processing tasks: %w",
			err,
		)
	}

	return taskList, nil
}

// SetStatus 设置任务状态。
func (p *Task) SetStatus(
	database *gorm.DB,
	taskID string,
	status TaskEnum,
) error {

	if database == nil {
		return errors.New("database is nil")
	}

	if !IsValidStatus(status) {
		return errors.New("invalid task status")
	}

	taskType, pos, err := ParseTaskID(taskID)
	if err != nil {
		return err
	}

	table, err := TaskTableName(taskType, pos)
	if err != nil {
		return err
	}

	return database.Table(table).
		Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"status": status,
		}).Error
}

// SetStatusAndRetryNumIncrement
// 更新任务状态，并且将重试次数加一。
func (p *Task) SetStatusAndRetryNumIncrement(
	database *gorm.DB,
	taskID string,
	status TaskEnum,
) error {

	if database == nil {
		return errors.New("database is nil")
	}

	if !IsValidStatus(status) {
		return errors.New("invalid task status")
	}

	taskType, pos, err := ParseTaskID(taskID)
	if err != nil {
		return err
	}

	table, err := TaskTableName(taskType, pos)
	if err != nil {
		return err
	}

	return database.Table(table).
		Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"status": status,
			"crt_retry_num": gorm.Expr(
				"crt_retry_num + ?", 1,
			),
		}).Error
}
