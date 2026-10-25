
# 第七章：完整 Worker 调度与 LarkTask

## 1. 对应原项目源码

- worker/src/tasksdk/schedule.go
- worker/src/main.go
- worker/src/tasksdk/schedule_test.go

## 2. Worker 启动

1. config.Init
2. tasksdk.InitSvr
3. tasksdk.RegisterHandler
4. 创建 TaskMgr
5. TaskMgr.ScheduleContext

## 3. 动态调度配置

Worker 通过 flowsvr 获取 t_schedule_cfg。

LoadCfg 在启动时加载配置。

CycleReloadCfg 每 20 秒尝试更新缓存。

TaskMgr 根据 ScheduleInterval 周期性调度。

轮询间隔增加 ±500ms 的随机波动。

## 4. 批量领取

按 task_type 加 Redis 分布式锁。

调用 HoldTasks 领取一批任务。

将 TaskData 转换为 TaskIntf。

本地状态设置为 processing。

领取结束立即释放 Redis 锁。

业务执行不持有 Redis 锁。

## 5. 并发控制

runTasks 使用：

- sync.WaitGroup
- 带缓冲的 channel 实现 Semaphore
- Goroutine

并发上限取 ScheduleLimit 和实际任务数中较小值。

当前批次执行完成以后，
TaskMgr 才会进行下一次调度。

## 6. 执行生命周期

run() 顺序：

1. ContextLoad
2. HandleProcess
3. 更新 ScheduleLog
4. 决定任务下一状态
5. 计算 OrderTime 或重试延迟
6. 处理业务收尾
7. SetTask 持久化

## 7. 状态管理

HandleProcess 返回 nil 不代表整个任务成功。

如果任务仍然处于 processing，
run 会将它改回 pending。

业务代码必须主动设置 success，
才能结束任务。

## 8. 重试

失败后：
- 更新 MaxRetryInterval
- 控制最大重试间隔
- 计算下次 OrderTime
- 增加 CrtRetryNum

MaxRetryNum=3 表示最多重试三次，
算上首次执行最多四次。

重试预算用完后任务进入 failed。

## 9. 调度日志

ScheduleLog 包含：
- LastData
- HistoryDatas

每轮记录：
- TraceId
- ErrMsg
- Cost

历史记录最多保留三条。

## 10. LarkTask

业务阶段：
sendmsg -> record

原示例使用 fallthrough，
因此可以在同一轮中直接完成。

目前是模拟任务，没有实际调用飞书 API。

## 11. 与原项目的局部差异

- 使用 RWMutex 保护配置缓存 map
- 增加 RunOnce 调试入口
- 增加信号控制的 Worker 停止方式
- LarkTask 增加空上下文错误检查
- 保留上一章 HTTP 标准库 RPC 客户端

核心任务调度和状态流转机制不变。

## 12. 当前仍有的问题

- MySQL Hold 不是原子领取
- SetTask 不检查任务执行权
- SetTask 的零值更新存在限制
- ContextLoad 失败没有正常消耗重试次数
- Panic 后可能遗留 processing 任务
- 部分失败收尾会阻止最终状态回写
- RunOnce 成功返回不保证每个 SetTask 都成功
- Worker 退出需要等待当前批次处理结束

后续任务治理章节继续实现：
超时任务恢复、滚动分表和调度位置推进。
