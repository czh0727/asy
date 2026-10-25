
# 第四章：任务注册、动态建表与任务创建

## 一、对应原始源码

- flowsvr/src/db/constant.go
- flowsvr/src/db/task.go
- flowsvr/src/db/task_pos.go
- flowsvr/src/db/task_schedule_cfg.go
- flowsvr/src/ctrl/ctrlmodel/model_convert.go
- flowsvr/src/ctrl/task/register_task.go
- flowsvr/src/ctrl/task/create_task.go
- flowsvr/src/cache/task.go

## 二、任务注册

POST /v1/register_task

执行步骤：

1. 按任务类型创建 t_{type}_task_1
2. 在 t_schedule_pos 初始化读写位置
3. 在 t_schedule_cfg 写入调度规则

注册步骤当前不是原子的，后续需要讨论
DDL 与元数据写入发生部分失败时的恢复策略。

## 三、任务创建

POST /v1/create_task

执行步骤：

1. 根据 task_type 获取分表位置
2. 根据 task_type 读取调度配置
3. 继承最大重试次数和初始重试间隔
4. 计算 order_time
5. 生成带任务类型和表位置的 task_id
6. 写入 MySQL 任务表
7. 写入 Redis 缓存
8. 返回 task_id

## 四、TaskId

格式：

UUID_task-type_position

例如：

UUID_video-generate_3

可反向定位：

t_video_generate_task_3

## 五、OrderTime

未来任务：
order_time = process_at

立即任务：
order_time = now - priority

priority 越大，order_time 越小。

后续调度按 order_time 升序领取任务。

## 六、MySQL 与 Redis

MySQL 是任务持久化数据源。

Redis 用于缓存任务数据。

创建任务时先写 MySQL，再写 Redis。
缓存写入失败不撤销已经成功的数据库插入。

## 七、本章明确保留的差异

- 限制 task_type 格式，保护动态 SQL 表名
- 查询索引后再决定是否创建索引
- 学习阶段的 HTTP 服务仅监听本地回环地址
- 增加纯 Go 单元测试及资源集成测试

## 八、尚未实现

- 批量 HoldTasks
- Redis 分布式锁
- Worker TaskMgr
- TaskIntf 与 TaskBase
- SetTask 状态回写
- 失败重试与任务阶段推进
- 任务滚动分表治理

## 九、下一章

基于原项目实现：

- HoldTasks HTTP 接口
- 获取待调度任务
- 批量设置 processing
- Redis 按 task_type 的分布式锁
- Worker 批量拉取任务
- 分析 Redis 锁与 MySQL 状态更新的并发风险
