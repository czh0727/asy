
# 第一章：任务模型与状态机

## 1. 背景问题

同步 HTTP 处理不适合直接承载长耗时任务。

主要问题：
- 请求可能超时
- 服务退出可能丢失任务
- 难以查询任务状态
- 失败后不方便重试

## 2. 当前方案

将业务任务抽象为独立的 Task 模型。

定义四个状态：

- pending：待执行
- processing：执行中
- success：执行成功
- failed：最终失败

允许的状态迁移：

pending -> processing

processing -> pending

processing -> success

processing -> failed

## 3. 设计取舍

使用独立 task 包定义领域规则。

原因：
- 不依赖 HTTP
- 不依赖数据库实现
- 方便单元测试
- 便于后续替换存储层

使用 crypto/rand 生成随机任务 ID。

使用 OrderTime 表示最早执行时间。

Priority 暂不参与排序。

## 4. 当前遗留问题

状态机不能保证数据库并发安全。

内存任务无法跨服务重启恢复。

暂未支持多 Worker 抢占。

暂未支持失败重试和超时恢复。

## 5. 下一阶段

引入 MySQL，持久化 Task。

通过数据库事务、条件更新实现任务状态原子迁移。
