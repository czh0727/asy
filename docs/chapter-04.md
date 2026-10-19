
# 第四章：Worker + TaskSDK

## 1. 业务背景

前面已经实现：
- Task 任务模型
- MySQL 持久化
- flowsvr HTTP 控制面

问题：任务必须手动调用 HTTP 接口才能执行。

## 2. 设计目标

实现独立 Worker 进程，自动完成：

1. 定时向 flowsvr 拉取任务
2. 根据 task_type 查找 Handler
3. 执行业务逻辑
4. 捕获 Handler 错误及 panic
5. 回写任务最终状态

## 3. 架构

Client -> flowsvr -> MySQL

Worker -> TaskSDK -> flowsvr

TaskSDK 包括：
- HTTPClient：与控制面通信
- Registry：业务 Handler 注册
- Worker：轮询和任务生命周期管理

## 4. 为什么不直接连接 MySQL？

通过 flowsvr 统一管理任务状态。

Worker 只关心业务执行，不关心表结构。

缺点是增加了网络通信与控制面负载。

## 5. 为什么现在串行执行？

先验证任务执行链路和状态流转的正确性。

下一阶段引入 Goroutine 和 Semaphore，
增加可配置并发数。

## 6. 错误处理

Handler 返回 nil：
  processing -> success

Handler 返回 error：
  processing -> failed

Handler panic：
  recover 后标记 failed

Worker 被取消：
  停止调度，未确认完成的任务可能留在 processing

HTTP Finish 失败：
  记录结果不确定的错误，后续通过恢复机制处理

## 7. 当前未解决的问题

- 没有失败重试
- 没有超时扫描和任务恢复
- 没有执行租约与 Fencing Token
- 没有保证业务副作用恰好一次
- 没有批量领取
- 没有任务并发执行池
- 还不能防止旧 Worker 回写新执行轮次

## 8. 下一章

多 Worker 与并发控制：

- Goroutine 并发执行
- Semaphore 限制并发数
- Context 和优雅退出
- Redis 协调的设计取舍
