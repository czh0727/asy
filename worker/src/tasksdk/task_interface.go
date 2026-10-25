package tasksdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/czh0727/asy/taskutils/constant"
	"github.com/czh0727/asy/taskutils/rpc"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/niuniumart/gosdk/tools"
)

// taskHandlerMap 保存业务任务类型和任务构造器的映射。
// 原项目使用普通 map，约定在启动调度前完成注册。
var taskHandlerMap = make(map[string]*TaskHandler)

// TaskHandler 定义一种业务任务的创建方式。
type TaskHandler struct {
	TaskType string
	NewProc  func() TaskIntf
}

// RegisterHandler 注册任务处理器。
func RegisterHandler(handler *TaskHandler) error {
	if err := validateHandler(handler); err != nil {
		return err
	}

	if _, exists := taskHandlerMap[handler.TaskType]; exists {
		return fmt.Errorf(
			"task handler %q already registered",
			handler.TaskType,
		)
	}

	taskHandlerMap[handler.TaskType] = handler
	return nil
}

// GetHandler 查询已注册的处理器。
func GetHandler(taskType string) (*TaskHandler, int) {
	if handler, ok := taskHandlerMap[taskType]; ok {
		return handler, 0
	}
	return nil, -1
}

func validateHandler(handler *TaskHandler) error {
	if handler == nil {
		return errors.New("task handler is nil")
	}
	if handler.TaskType == "" {
		return errors.New("task handler task type is empty")
	}
	if handler.NewProc == nil {
		return fmt.Errorf(
			"task handler %q new proc is nil",
			handler.TaskType,
		)
	}
	return nil
}

// TaskIntf 是原项目的业务任务生命周期接口。
type TaskIntf interface {
	ContextLoad() error
	HandleProcess() error
	SetTask() error
	HandleFinish()
	HandleFinishError() error
	Base() *TaskBase
	CreateTask() (string, error)
	HandleFailedMust() error
}

// TaskBase 保存所有业务任务共用的字段。
type TaskBase struct {
	Id               uint64
	TaskId           string
	UserId           string
	Status           int
	TaskType         string
	TaskStage        string
	TaskContext      string
	CrtRetryNum      int
	MaxRetryNum      int
	MaxRetryInterval int
	ScheduleLog      *ScheduleLog
	ContextIntf      interface{}
	Priority         int64
	OrderTime        int64
	CreateTime       time.Time
	ModifyTime       time.Time
}

// ScheduleLog 保存任务本轮和历史调度信息。
type ScheduleLog struct {
	LastData     ScheduleData
	HistoryDatas []ScheduleData
}

// ScheduleData 保存一次调度的执行记录。
type ScheduleData struct {
	TraceId string
	ErrMsg  string
	Cost    string
}

// Base 返回当前任务的基础信息。
func (p *TaskBase) Base() *TaskBase {
	return p
}

// SetContextLocal 在内存中更新业务上下文。
// 真正持久化由 SetTask 负责。
func (p *TaskBase) SetContextLocal(data interface{}) {
	p.ContextIntf = data
}

// GetTaskInfoFromStorage 将 RPC TaskData 转换为业务任务对象。
func GetTaskInfoFromStorage(
	storage *model.TaskData,
) (TaskIntf, error) {
	if storage == nil {
		return nil, errors.New("storage task data is nil")
	}
	if storage.TaskType == "" {
		return nil, errors.New("storage task type is empty")
	}

	handler, ok := taskHandlerMap[storage.TaskType]
	if !ok {
		return nil, fmt.Errorf(
			"task type %q not exist in taskHandlerMap",
			storage.TaskType,
		)
	}

	if err := validateHandler(handler); err != nil {
		return nil, err
	}

	// 不同任务类型通过不同构造器生成业务实例。
	task := handler.NewProc()
	if task == nil {
		return nil, fmt.Errorf(
			"task handler %q returned nil task",
			storage.TaskType,
		)
	}

	base := task.Base()
	if base == nil {
		return nil, fmt.Errorf(
			"task handler %q returned nil base",
			storage.TaskType,
		)
	}

	// 还原任务基本信息。
	base.TaskId = storage.TaskId
	base.TaskType = storage.TaskType
	base.TaskStage = storage.TaskStage
	base.TaskContext = storage.TaskContext
	base.Status = storage.Status
	base.UserId = storage.UserId

	if storage.Priority != nil {
		base.Priority = int64(*storage.Priority)
	}

	base.CrtRetryNum = storage.CrtRetryNum
	base.MaxRetryNum = storage.MaxRetryNum
	base.MaxRetryInterval = storage.MaxRetryInterval
	base.CreateTime = storage.CreateTime
	base.ModifyTime = storage.ModifyTime

	// 还原原项目中的调度日志。
	// 原源码忽略日志 JSON 解析错误，此处暂时保持行为。
	var info ScheduleLog
	if storage.ScheduleLog != "" {
		_ = json.Unmarshal(
			[]byte(storage.ScheduleLog),
			&info,
		)
	}
	base.ScheduleLog = &info

	// 原项目此处未复制 OrderTime。
	// 后续业务调度的 run() 会重新计算 OrderTime。
	return task, nil
}

// HandleFinishError 是可以失败的业务收尾操作。
func (p *TaskBase) HandleFinishError() error {
	return nil
}

// HandleFailedMust 是最终失败时的必要操作。
// 由具体业务按需覆盖。
func (p *TaskBase) HandleFailedMust() error {
	return nil
}

// 保留原项目提供的辅助方法。
func (p *TaskBase) HandleFinishMust() error {
	return nil
}

// ContextLoad 的默认行为是不做处理。
// 具体业务任务可覆盖它，反序列化 TaskContext。
func (p *TaskBase) ContextLoad() error {
	return nil
}

// SetTask 将 TaskBase 的状态和上下文提交到 flowsvr。
func (p *TaskBase) SetTask() error {
	if p == nil {
		return errors.New("task base is nil")
	}

	client := rpc.TaskRpc{
		Host: taskSvrHost,
	}

	taskData := model.TaskData{
		TaskType:         p.TaskType,
		TaskId:           p.TaskId,
		TaskStage:        p.TaskStage,
		TaskContext:      p.TaskContext,
		ScheduleLog:      tools.GetFmtStr(p.ScheduleLog),
		UserId:           p.UserId,
		CrtRetryNum:      p.CrtRetryNum,
		MaxRetryNum:      p.MaxRetryNum,
		MaxRetryInterval: p.MaxRetryInterval,
		Status:           p.Status,
		OrderTime:        p.OrderTime,
	}

	// 如果业务通过 SetContextLocal 修改了上下文，
	// 需要将最新的对象序列化到 context 字段。
	if p.ContextIntf != nil {
		data, err := json.Marshal(p.ContextIntf)
		if err != nil {
			return fmt.Errorf(
				"marshal task context: %w",
				err,
			)
		}
		taskData.TaskContext = string(data)
	}

	_, err := client.SetTask(
		&model.SetTaskReq{
			TaskData: taskData,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"set task %s: %w",
			p.TaskId,
			err,
		)
	}

	return nil
}

// CreateTask 通过 flowsvr 创建一条新任务。
// 本章保留原始 TaskBase.CreateTask 的字段传递方式。
func (p *TaskBase) CreateTask() (string, error) {
	if p == nil {
		return "", errors.New("task base is nil")
	}

	taskData := model.TaskData{
		TaskType:         p.TaskType,
		TaskContext:      p.TaskContext,
		UserId:           p.UserId,
		MaxRetryNum:      p.MaxRetryNum,
		MaxRetryInterval: p.MaxRetryInterval,
		Status:           p.Status,
	}

	if taskData.Status == 0 {
		taskData.Status = int(
			constant.TASK_STATUS_PENDING,
		)
	}

	client := rpc.TaskRpc{
		Host: taskSvrHost,
	}

	resp, err := client.CreateTask(
		&model.CreateTaskReq{
			TaskData: taskData,
		},
	)
	if err != nil {
		return "", err
	}

	return resp.TaskId, nil
}

// GetTask 从 flowsvr 读取任务，再还原为 TaskIntf。
func GetTask(taskID string) (TaskIntf, error) {
	client := rpc.TaskRpc{
		Host: taskSvrHost,
	}

	resp, err := client.GetTask(
		&model.GetTaskReq{
			TaskId: taskID,
		},
	)
	if err != nil {
		return nil, err
	}

	return GetTaskInfoFromStorage(resp.TaskData)
}
