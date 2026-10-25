package db

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jinzhu/gorm"
)

var TaskPosNsp TaskPos

type TaskPos struct {
	Id               uint64
	TaskType         string
	ScheduleBeginPos int
	ScheduleEndPos   int
	CreateTime       *time.Time
	ModifyTime       *time.Time
}

func (p *TaskPos) TableName() string {
	return "t_schedule_pos"
}

func (p *TaskPos) Create(
	database *gorm.DB,
	value *TaskPos,
) error {
	return database.Table(p.TableName()).
		Create(value).Error
}

func (p *TaskPos) Save(
	database *gorm.DB,
	value *TaskPos,
) error {
	return database.Table(p.TableName()).
		Save(value).Error
}

// GetTaskPos 获取任务类型的读写游标。
// 保留原项目：若尚无游标，则尝试初始化为 (1,1)。
func (p *TaskPos) GetTaskPos(
	database *gorm.DB,
	taskType string,
) (*TaskPos, error) {
	if err := ValidateTaskType(taskType); err != nil {
		return nil, err
	}

	var position TaskPos
	err := database.Table(p.TableName()).
		Where("task_type = ?", taskType).
		First(&position).Error

	if err == gorm.ErrRecordNotFound {
		position = TaskPos{
			TaskType:         taskType,
			ScheduleBeginPos: 1,
			ScheduleEndPos:   1,
		}

		err = database.Table(p.TableName()).
			Create(&position).Error
	}

	if err != nil {
		return nil, err
	}

	return &position, nil
}

func (p *TaskPos) GetNextPos(pos string) string {
	n, err := strconv.Atoi(pos)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d", n+1)
}

func (p *TaskPos) GetTaskPosList(
	database *gorm.DB,
) ([]*TaskPos, error) {
	list := make([]*TaskPos, 0)

	err := database.Table(p.TableName()).
		Find(&list).Error

	return list, err
}

func (p *TaskPos) BeforeCreate(
	scope *gorm.Scope,
) error {
	now := time.Now()
	if err := scope.SetColumn("create_time", now); err != nil {
		return err
	}
	return scope.SetColumn("modify_time", now)
}
