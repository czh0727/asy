
# 第六章：失败重试与指数退避

## 1. 背景问题

Worker 执行任务可能发生临时性错误。

直接将任务标记为 failed 会降低任务成功率。

在 Worker 中循环 Sleep 重试会长期占用执行槽位，
并且 Worker 重启后会丢失内存中的重试状态。

## 2. 方案演进

方案一：Worker 内立即重试
问题：占用槽位、可能高频冲击下游、不可持久恢复。

方案二：Worker Sleep 后重试
问题：仍然占用槽位，重启后丢失等待状态。

最终方案：MySQL 持久化延迟重试。

## 3. 状态转换

可重试错误：

processing -> pending

同时：
retry_count += 1
order_time = 数据库当前时间 + 指数退避时间

当重试额度耗尽时：

processing -> failed

不可重试错误：

processing -> failed

## 4. 重试次数语义

max_retries 表示额外重试次数。

max_retries=3，最多执行 4 次。

retry_count 表示已经安排的额外重试次数。

## 5. 退避策略

2s、4s、8s、16s、32s、60s ...

最大等待 60s。

当前没有增加随机抖动。

## 6. 原子性

使用 MySQL 事务和 SELECT FOR UPDATE。

在同一个事务中完成：
- 读取 retry_count
- 检查 max_retries
- 修改 status
- 修改 retry_count
- 修改 order_time

## 7. 错误分类

普通 error：永久失败。

Retryable(error)：允许重试。

由 Handler 根据业务语义决定是否可重试。

## 8. 已知局限

- 缺少 last_error 的持久记录
- 未实现指数退避的随机抖动
- 未实现超时任务扫描
- 未实现 Lease / Fencing Token
- 未实现业务副作用幂等
- 尚未实现 API 调用级别的幂等键

## 9. 下一章

超时恢复、Lease 和执行轮次隔离。

重点解决：
- Worker 领取后宕机
- processing 任务长时间无法恢复
- 旧 Worker 覆盖新 Worker 状态
