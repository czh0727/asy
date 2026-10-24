
# 第二章：MySQL、Redis 初始化与数据库结构

## 1. 对应原项目源码

- flowsvr/src/db/db.go
- flowsvr/src/cache/cache.go
- flowsvr/src/initialize/initialize.go
- sql/create.sql

## 2. 本章目标

完成 flowsvr 基础资源初始化。

启动顺序：

config.Init()
    ↓
initialize.InitResource()
    ↓
db.InitDB()
    ↓
cache.InitCache()

## 3. MySQL

原项目使用：

- Gorm v1
- gosdk/gormcli
- MySQL 连接池
- Ping 检查数据库可用性

MySQL 保存任务事实和调度元数据。

## 4. Redis

原项目使用：

- go-redis/v9
- gosdk/goredis
- 带过期时间的任务缓存

后续 Worker 使用 Redis 分布式锁。

## 5. 原始数据库表

t_lark_task_1：
保存 lark 类型任务。

t_schedule_cfg：
保存每种任务的调度参数。

t_schedule_pos：
保存任务读取和写入的分表位置。

初始数据：

task_type = lark
schedule_limit = 100
max_retry_num = 3
schedule_begin_pos = 1
schedule_end_pos = 1

## 6. 与原项目的差异

为便于本地开发：
- 使用 Docker Compose 启动 MySQL、Redis
- MySQL 本地端口使用 3307
- Redis 本地端口使用 6380
- SQL 增加幂等初始化保护
- 本地密码仅用于开发
- 增加独立的资源集成测试
- 增加资源关闭和 Ping 辅助方法

任务表字段、调度配置字段和核心资源初始化结构
均按照原项目实现。

## 7. 本章暂未实现

- Task 模型与 Gorm 映射
- 任务类型注册
- TaskContext 和 ScheduleLog
- HTTP API
- Redis 任务缓存读写
- Worker Redis 分布式锁
- 批量 Hold
- 任务执行及失败重试
- 滚动分表和超时治理

## 8. 下一章

实现 taskutils/rpc/model：

- 任务状态常量
- TaskData
- CreateTaskReq / CreateTaskResp
- HoldTasksReq / HoldTasksResp
- SetTaskReq / SetTaskResp
- TaskScheduleCfg

为原项目 HTTP 接口与 TaskSDK 奠定协议基础。
