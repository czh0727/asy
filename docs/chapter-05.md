
# 第五章：HoldTasks 与 Redis 分布式锁

## 一、原项目对应源码

- flowsvr/src/ctrl/task/hold_tasks.go
- flowsvr/src/db/task.go
- flowsvr/src/ctrl/task/get_task_schedule_cfg_list.go
- taskutils/rpc/task_rpc.go
- worker/src/tasksdk/schedule.go

## 二、批量领取流程

1. Worker 获取 task_type 对应 Redis 锁
2. Worker 调用 TaskRpc.HoldTasks
3. flowsvr 根据 task_type 查找 ScheduleBeginPos
4. 从相应任务表查询 pending 任务
5. 过滤尚未到达 order_time 的任务
6. 按 order_time 升序读取指定数量
7. 批量将任务状态设置为 processing
8. 返回 TaskData 列表
9. Worker 释放 Redis 锁

## 三、领取数量

服务端：
- 默认最大领取数量 2000
- 超过 2000 时截断为 2000

Worker：
- 从 t_schedule_cfg 读取 ScheduleLimit
- 本章没有配置时使用 SCHEDULE_TASK_LIMIT

## 四、Redis 锁

锁的 key 使用 task_type。

原项目参数：
- WithBlock
- WithWatchDogMode
- WithExpireSeconds(3)

锁仅保护批量领取过程，不保护业务执行过程。

## 五、MySQL 查询条件

task_type = 指定任务类型
status = pending
order_time < 当前 Unix 秒

ORDER BY order_time ASC

## 六、重要一致性问题

原始方案的 GetTaskList 和 BatchSetStatus
不在一个原子领取事务中。

Redis 锁可以协调遵守协议的 Worker，
但不能消除绕过锁请求、锁过期、进程暂停等风险。

本章先复现原始方案，不将其描述为 Exactly-once。

## 七、当前实现范围

已实现：
- HoldTasks HTTP
- GetTaskScheduleCfgList HTTP
- TaskRpc HTTP 客户端
- Redis 按任务类型互斥
- 一次性 Worker 批量 Hold
- MySQL 顺序领取测试

暂未实现：
- TaskIntf
- TaskBase
- TaskMgr 周期性调度
- Handler 并发执行
- SetTask 状态回写
- 崩溃恢复与滚动分表

## 八、下一章

重建原项目 TaskSDK：
- TaskIntf
- TaskBase
- TaskStage
- TaskContext
- ScheduleLog
- 任务类型注册与存储任务转换
- SetTask RPC

然后再把 HoldOnce 改造成周期性调度和业务执行。
