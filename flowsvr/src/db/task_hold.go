package db

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
)

// GetTaskList 获取一批满足调度条件的任务。
//
// 保留原项目的筛选语义：
// task_type 相同
// status = pending
// order_time < 当前 Unix 秒
// 按 order_time 升序
func (p *Task) GetTaskList(
	database *gorm.DB,
	pos string,
	taskType string,
	status TaskEnum,
	limit int,
) ([]*Task, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be positive")
	}

	n, err := strconv.Atoi(pos)
	if err != nil {
		return nil, err
	}

	table, err := TaskTableName(taskType, n)
	if err != nil {
		return nil, err
	}

	list := make([]*Task, 0)

	err = database.Table(table).
		Where("task_type = ?", taskType).
		Where("status = ?", status).
		Where("order_time < ?", time.Now().Unix()).
		Order("order_time ASC").
		Limit(limit).
		Find(&list).Error

	if err != nil {
		return nil, fmt.Errorf(
			"get task list: %w", err,
		)
	}

	return list, nil
}

// BatchSetStatus 对一批同一物理表中的任务设置状态。
//
// 这一步保留原始项目的实现方式：
// 查询任务和批量更新不是同一个事务中的原子领取。
func (p *Task) BatchSetStatus(
	database *gorm.DB,
	taskIDs []string,
	status TaskEnum,
) error {
	if len(taskIDs) == 0 {
		return nil
	}

	if !IsValidStatus(status) {
		return errors.New("invalid task status")
	}

	taskType, pos, err := ParseTaskID(taskIDs[0])
	if err != nil {
		return err
	}

	// 本方法一次只处理同一物理表的任务。
	for _, id := range taskIDs[1:] {
		t, n, err := ParseTaskID(id)
		if err != nil {
			return err
		}
		if t != taskType || n != pos {
			return errors.New(
				"batch tasks must belong to the same table",
			)
		}
	}

	table, err := TaskTableName(taskType, pos)
	if err != nil {
		return err
	}

	fields := map[string]interface{}{
		"status": status,
		"modify_time": time.Now().Format(
			"2006-01-02 15:04:05",
		),
	}

	return database.Table(table).
		Where("task_id IN (?)", taskIDs).
		UpdateColumns(fields).Error
}
