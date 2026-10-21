
# 第五章：多 Worker 协同与有界并发

## 一、背景问题

第四章 Worker 串行执行任务。

当存在大量耗时任务时，整体吞吐量受到限制。

直接为每个任务启动 Goroutine，可能导致无界并发，
消耗过多内存、连接和外部 API 配额。

另外，Worker 收到退出信号时，
当前任务可能被直接取消，停留在 processing。

## 二、解决方案

使用带缓冲 Channel 实现 Semaphore。

每个 Worker 配置：
- MaxConcurrent：本地最大并发任务数
- ShutdownGrace：优雅退出宽限期

执行流程：
1. 获取空闲槽位
2. 调用 flowsvr Claim
3. 没有任务则释放槽位
4. 领取成功后启动 Goroutine
5. Handler 执行并回写结果
6. 释放槽位

## 三、为什么先占槽位再 Claim？

如果先 Claim 再等 Semaphore，
任务可能已经变成 processing，
但一直没有得到本地执行资源。

先获取槽位可以避免此类本地积压。

## 四、多 Worker 为什么能并行领取？

第二章的 MySQL ClaimNext 使用：

- READ COMMITTED 事务
- SELECT FOR UPDATE SKIP LOCKED
- 条件 UPDATE
- RowsAffected
- COMMIT

不同 Worker 通过 flowsvr 领取任务。
MySQL 负责状态抢占的原子性。

## 五、为什么暂不增加 Redis 分布式锁？

当前 MySQL 已经提供原子领取机制。

如果按 task_type 增加全局 Redis 锁，
可能将不同 Worker 的 Claim 串行化。

未来实现批量领取与调度协调时，
再评估 Redis 锁的具体用途。

Redis 锁不能保证业务 Exactly-once。

## 六、优雅退出设计

收到停止信号：
1. 停止领取新任务
2. 保留已执行任务的 Context
3. 等待 WaitGroup
4. 超过 ShutdownGrace 后取消任务 Context
5. 未完成任务暂时可能留在 processing

## 七、当前限制

- 不支持失败自动重试
- 不支持处理超时后自动恢复
- 不支持 Lease / Fencing Token
- 不保证 Exactly-once 业务执行
- 不支持批量领取
- Handler 必须自行保证并发安全
- 多实例总并发上限尚未实现全局协调

## 八、下一步

实现失败重试、延迟调度及指数退避，
再实现超时恢复与执行轮次隔离。
