
# 第六章：TaskIntf、TaskBase 与任务状态回写

## 1. 对应原项目源码

- worker/src/tasksdk/task_interface.go
- flowsvr/src/ctrl/task/set_task.go
- flowsvr/src/ctrl/task/get_task.go
- flowsvr/src/db/task.go
- taskutils/rpc/task_rpc.go

## 2. 本章目标

实现 Asyncflow 原始 TaskSDK 任务抽象：

- TaskIntf
- TaskHandler
- TaskBase
- ScheduleLog
- ScheduleData
- RegisterHandler
- GetTaskInfoFromStorage

同时实现 SetTask、GetTask RPC 和持久化。

## 3. TaskIntf

原始接口：

ContextLoad() error
HandleProcess() error
SetTask() error
HandleFinish()
HandleFinishError() error
Base() *TaskBase
CreateTask() (string, error)
HandleFailedMust() error

各业务任务通过嵌入 TaskBase 复用共用能力。

## 4. TaskHandler

TaskHandler 使用：

TaskType string
NewProc func() TaskIntf

TaskSDK 根据任务类型创建对应业务实例。

任务注册应在启动调度前完成。

## 5. TaskContext

数据库使用 task_context 字段保存字符串。

RPC 使用 context 字段传输字符串。

业务通过 ContextLoad 反序列化。

业务可以通过 SetContextLocal 修改本地上下文。

TaskBase.SetTask 将 ContextIntf 转为 JSON 字符串，
再通过 RPC 提交给 flowsvr。

## 6. TaskStage

TaskStage 是字符串，不是独立状态枚举。

例如：
sendmsg -> record

它表示业务处理阶段，与四种任务状态不同。

## 7. ScheduleLog

记录：

- LastData
- HistoryDatas
- TraceId
- ErrMsg
- Cost

本章只实现结构和序列化。

下一章实现 run() 后才会真正追加执行历史。

## 8. SetTask

Worker：
TaskBase.SetTask()

HTTP：
POST /v1/set_task

flowsvr：
SetTaskHandler.HandleProcess()

数据库：
db.Task.UpdateTask()

缓存：
cache.OnceDeleteTask()

## 9. GetTask

GET /v1/get_task?task_id=...

先查 Redis。
未命中再查 MySQL。
查询成功后尝试回填缓存。

## 10. 与原项目的明确差异

- 补充 TaskId 与 TaskType 一致性校验
- 补充合法任务状态校验
- 修正 GetTask 客户端查询参数名
- 增加任务生命周期与持久化测试
- HTTP 客户端继续使用前一章的标准库封装

原始 TaskIntf、TaskBase 和 RPC 字段语义不变。

## 11. 保留的原项目限制

- UpdateTask 使用 Gorm v1 的 Updates(struct)
- 零值字段可能被忽略
- 不检查 RowsAffected
- SetTask 不具有 Worker 执行权验证
- GetTaskInfoFromStorage 未复制 OrderTime
- TaskBase.CreateTask 未转发所有任务字段
- ScheduleLog 解析失败暂时不返回错误
- 注册处理器使用普通 map

这些限制将在整个项目运行后统一分析。

## 12. 下一章

实现原始 Worker 调度与 LarkTask：

- TaskMgr.Schedule
- ScheduleContext
- CycleReloadCfg
- LoadCfg
- schedule / hold / runTasks
- Semaphore
- WaitGroup
- run() 生命周期
- LarkTask
- 执行阶段推进、重试、调度日志记录
