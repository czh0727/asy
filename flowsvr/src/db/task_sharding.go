package db

import (
	"errors"
	"strconv"

	"github.com/jinzhu/gorm"
)

// GetAllTaskCount 获取指定分表的所有任务数量。
func (p *Task) GetAllTaskCount(
	database *gorm.DB,
	taskType string,
	pos string,
) (int, error) {

	if database == nil {
		return 0, errors.New("database is nil")
	}

	position, err := strconv.Atoi(pos)
	if err != nil {
		return 0, err
	}

	table, err := TaskTableName(taskType, position)
	if err != nil {
		return 0, err
	}

	var count int

	err = database.Table(table).
		Count(&count).Error

	return count, err
}

// GetFinishTaskCount 获取已经完成的任务数量。
// success 和 failed 都属于终态。
func (p *Task) GetFinishTaskCount(
	database *gorm.DB,
	taskType string,
	pos string,
) (int, error) {

	if database == nil {
		return 0, errors.New("database is nil")
	}

	position, err := strconv.Atoi(pos)
	if err != nil {
		return 0, err
	}

	table, err := TaskTableName(taskType, position)
	if err != nil {
		return 0, err
	}

	var count int

	err = database.Table(table).
		Where(
			"status IN (?)",
			[]TaskEnum{
				TASK_STATUS_SUCCESS,
				TASK_STATUS_FAILED,
			},
		).
		Count(&count).Error

	return count, err
}
