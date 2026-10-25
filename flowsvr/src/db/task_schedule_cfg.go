package db

import (
	"time"

	"github.com/jinzhu/gorm"
)

var TaskTypeCfgNsp TaskScheduleCfg

type TaskScheduleCfg struct {
	TaskType          string
	ScheduleLimit     int
	ScheduleInterval  int
	MaxProcessingTime int64
	MaxRetryNum       int
	RetryInterval     int
	MaxRetryInterval  int
	CreateTime        *time.Time
	ModifyTime        *time.Time
}

func (p *TaskScheduleCfg) TableName() string {
	return "t_schedule_cfg"
}

func (p *TaskScheduleCfg) Create(
	database *gorm.DB,
	value *TaskScheduleCfg,
) error {
	return database.Table(p.TableName()).
		Create(value).Error
}

func (p *TaskScheduleCfg) Save(
	database *gorm.DB,
	value *TaskScheduleCfg,
) error {
	return database.Table(p.TableName()).
		Save(value).Error
}

func (p *TaskScheduleCfg) GetTaskTypeCfg(
	database *gorm.DB,
	taskType string,
) (*TaskScheduleCfg, error) {
	var cfg TaskScheduleCfg

	err := database.Table(p.TableName()).
		Where("task_type = ?", taskType).
		First(&cfg).Error

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (p *TaskScheduleCfg) GetTaskTypeCfgList(
	database *gorm.DB,
) ([]*TaskScheduleCfg, error) {
	list := make([]*TaskScheduleCfg, 0)

	err := database.Table(p.TableName()).
		Find(&list).Error

	return list, err
}
