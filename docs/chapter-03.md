
# 第三章：任务模型与 RPC 协议

## 一、对应原项目

- taskutils/constant/constant.go
- taskutils/rpc/model/constant.go
- taskutils/rpc/model/task_model.go
- flowsvr/src/constant/error.go
- flowsvr/src/constant/http.go
- flowsvr/src/constant/task.go

## 二、核心目标

实现 Asyncflow 原始任务状态、
HTTP 路径常量、RPC 请求与响应结构。

不额外设计新的状态机或任务协议。

## 三、任务状态

1 = pending
2 = processing
3 = success
4 = failed

## 四、TaskData

TaskData 是 Worker 与 flowsvr 之间的
任务数据传输结构，而不是 Gorm 数据库模型。

重要字段：

- TaskId：任务唯一标识
- TaskType：任务类型
- TaskStage：业务执行阶段
- TaskContext：业务上下文
- Status：任务状态
- Priority：优先级指针
- CrtRetryNum：当前重试次数
- MaxRetryNum：最大重试次数
- MaxRetryInterval：当前最大重试间隔
- ScheduleLog：调度日志
- OrderTime：排序时间
- ProcessAt：计划执行时间

## 五、协议注意事项

- TaskContext 使用 JSON 字段 context。
- TaskContext 本身是字符串。
- ScheduleLog 使用字符串传输。
- Priority 是 *int，可以区分 nil 和 0。
- SetTaskReq 中 TaskData 使用嵌套 JSON。
- RespComm 提供 code 和 msg。
- HoldTasksResp 返回任务列表。

## 六、暂未实现

- MySQL Task Gorm 模型
- RPC HTTP 客户端
- flowsvr HTTP Handler
- 任务类型注册
- 批量任务领取
- Worker TaskSDK

## 七、下一章

按照原项目实现：

- db.Task
- db.TaskPos
- db.TaskScheduleCfg
- TaskId 生成与表位置解析
- RegisterTask
- CreateTask
- 数据库实体与 TaskData 转换
