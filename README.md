
# AsyncTiger V2

基于 Go 实现的分布式异步任务调度框架。

目标架构：

- flowsvr：任务控制面
- Worker / TaskSDK：任务执行面
- MySQL：任务持久化
- Redis：分布式协调

## 第一章：任务模型与状态机

当前实现：

- Task 领域模型
- pending / processing / success / failed 状态机
- 任务创建与参数校验
- Runnable 调度条件判断
- 单元测试

## 运行

go test ./...

go test -race ./...

go run ./cmd/demo

当前阶段尚未实现 MySQL、Redis、HTTP 接口及分布式调度。
