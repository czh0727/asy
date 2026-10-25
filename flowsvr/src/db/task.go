package db

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

var TaskNsp Task

// Task 对应 t_{taskType}_task_{pos} 的记录。
type Task struct {
	Id               int       `gorm:"primary_key;AUTO_INCREMENT"`
	UserId           string    `gorm:"type:varchar(256);column:user_id;not null;default:'';index:idx_user_id"`
	TaskId           string    `gorm:"type:varchar(256);column:task_id;not null;default:'';unique_index:idx_task_id"`
	TaskType         string    `gorm:"type:varchar(128);column:task_type;not null"`
	TaskStage        string    `gorm:"type:varchar(128);column:task_stage;not null"`
	Status           int       `gorm:"type:tinyint(3);column:status;not null;default:1;index:idx_tasktype_status_modify_time"`
	Priority         int       `gorm:"type:int(11);column:priority;not null;default:0;comment:'优先级'"`
	CrtRetryNum      int       `gorm:"type:int(11);column:crt_retry_num;not null;default:0"`
	MaxRetryNum      int       `gorm:"type:int(11);column:max_retry_num;not null;default:0"`
	MaxRetryInterval int       `gorm:"type:int(11);column:max_retry_interval;not null;default:0"`
	ScheduleLog      string    `gorm:"type:varchar(4096);column:schedule_log;not null;default:''"`
	TaskContext      string    `gorm:"type:varchar(8192);column:task_context;not null;default:''"`
	OrderTime        int64     `gorm:"type:int(20);column:order_time;not null;default:0;index:idx_tasktype_status_modify_time"`
	CreateTime       time.Time `gorm:"type:datetime;column:create_time;not null"`
	ModifyTime       time.Time `gorm:"type:datetime;column:modify_time;not null"`
}

func (p *Task) TableName() string {
	return "task"
}

// 保护动态 SQL 表名：只允许安全的任务类型。
// 这是本次重建增加的输入限制。
var taskTypePattern = regexp.MustCompile(
	`^[a-z][a-z0-9_]{0,47}$`,
)

func ValidateTaskType(taskType string) error {
	if !taskTypePattern.MatchString(taskType) {
		return errors.New(
			"task_type must start with a lowercase letter " +
				"and contain only lowercase letters, numbers, " +
				"or underscores (max 48 characters)",
		)
	}
	return nil
}

// TaskTableName 生成原项目格式的物理表名。
func TaskTableName(taskType string, pos int) (string, error) {
	if err := ValidateTaskType(taskType); err != nil {
		return "", err
	}
	if pos <= 0 {
		return "", errors.New("table position must be positive")
	}

	name := fmt.Sprintf("t_%s_task_%d", taskType, pos)
	if len(name) > 64 {
		return "", errors.New("mysql table name too long")
	}
	return name, nil
}

// GenTaskId 生成：UUID_任务类型_分表位置。
// 任务类型中的下划线替换成连字符。
func (p *Task) GenTaskId(taskType, pos string) string {
	encodedType := strings.ReplaceAll(taskType, "_", "-")
	return fmt.Sprintf("%s_%s_%s", uuid.NewString(), encodedType, pos)
}

// ParseTaskID 从任务 ID 反向解析任务类型和分表位置。
func ParseTaskID(taskID string) (string, int, error) {
	parts := strings.Split(taskID, "_")
	if len(parts) != 3 {
		return "", 0, errors.New("invalid task id format")
	}

	if _, err := uuid.Parse(parts[0]); err != nil {
		return "", 0, fmt.Errorf("invalid task uuid: %w", err)
	}

	taskType := strings.ReplaceAll(parts[1], "-", "_")
	pos, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid task position: %w", err)
	}

	if _, err := TaskTableName(taskType, pos); err != nil {
		return "", 0, err
	}

	return taskType, pos, nil
}

// CreateTable 为新任务类型创建第一张任务表。
// 使用 Gorm v1 的 AutoMigrate，保留原项目行为。
func (p *Task) CreateTable(
	database *gorm.DB,
	taskType, pos string,
) error {
	position, err := strconv.Atoi(pos)
	if err != nil {
		return err
	}

	table, err := TaskTableName(taskType, position)
	if err != nil {
		return err
	}

	if err := database.Table(table).
		AutoMigrate(&Task{}).Error; err != nil {
		return err
	}

	// 保留原项目的任务类型、状态、修改时间索引。
	// 注册前先查看该索引是否存在，避免重复创建。
	const indexName = "idx_task_type_status_modify_time"

	var indexCount int
	if err := database.DB().QueryRow(
		`SELECT COUNT(*)
		 FROM information_schema.statistics
		 WHERE table_schema = DATABASE()
		   AND table_name = ?
		   AND index_name = ?`,
		table,
		indexName,
	).Scan(&indexCount); err != nil {
		return fmt.Errorf("check index %s on %s: %w", indexName, table, err)
	}
	if indexCount > 0 {
		return nil
	}

	return database.Table(table).AddIndex(
		indexName,
		"task_type",
		"status",
		"modify_time",
	).Error
}

// Create 创建一条业务任务。
func (p *Task) Create(
	database *gorm.DB,
	taskType, pos string,
	value *Task,
) error {
	if value == nil {
		return errors.New("nil task")
	}
	if value.TaskType != taskType {
		return errors.New("task type mismatch")
	}

	position, err := strconv.Atoi(pos)
	if err != nil {
		return err
	}

	table, err := TaskTableName(taskType, position)
	if err != nil {
		return err
	}

	return database.Table(table).Create(value).Error
}

// Find 按 TaskId 查询任务。
func (p *Task) Find(
	database *gorm.DB,
	taskID string,
) (*Task, error) {
	taskType, pos, err := ParseTaskID(taskID)
	if err != nil {
		return nil, err
	}

	table, err := TaskTableName(taskType, pos)
	if err != nil {
		return nil, err
	}

	var result Task
	err = database.Table(table).
		Where("task_id = ?", taskID).
		First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// BeforeCreate 在插入任务时初始化时间。
func (p *Task) BeforeCreate(scope *gorm.Scope) error {
	now := time.Now()
	if err := scope.SetColumn("create_time", now); err != nil {
		return err
	}
	return scope.SetColumn("modify_time", now)
}
