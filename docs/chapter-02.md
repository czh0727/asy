
# 第二章：MySQL 持久化与原子领取

## 1. 业务背景

第一章任务只存在于 Go 内存中。

问题：
- 服务退出后无法找回任务
- 多 Worker 可能领取同一条任务
- 状态更新缺少持久化保证

## 2. 方案选择

使用 MySQL InnoDB 保存任务事实。

任务创建：INSERT

任务查询：SELECT

任务领取：
1. 开启 READ COMMITTED 事务
2. SELECT FOR UPDATE SKIP LOCKED
3. 条件 UPDATE pending -> processing
4. 检查 RowsAffected
5. COMMIT

任务完成：
条件 UPDATE processing -> success/failed

## 3. 为什么这样设计

普通 SELECT + UPDATE 存在并发竞争。

SELECT FOR UPDATE 在事务内锁定行。

SKIP LOCKED 跳过其他事务已锁定的记录。

条件 UPDATE 再次校验状态，防止错误迁移。

事务提交前不返回领取成功。

## 4. 为什么暂时不使用 Redis 锁

MySQL 是任务事实的最终存储。

先保证数据库内部状态迁移的原子性。

Redis 后续用于分布式协调、减少竞争，
但不能代替数据库的条件状态更新。

## 5. 当前局限性

- 不能保证业务 Handler 恰好执行一次
- 尚未实现 Worker 身份校验和 Fencing Token
- 尚未实现超时恢复、心跳和失败重试
- SKIP LOCKED 不保证严格全局 FIFO
- 还没有实现批量任务领取
- 优先级调度策略仍需要进一步完善

## 6. 本章测试

- Create / Get / Finish
- 重复任务 ID
- 重复完成任务
- 未到期任务领取
- 12 个并发请求争抢同一任务
