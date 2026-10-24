
# asy — Asyncflow 从零重建

原项目：
https://github.com/czh0727/asyn

本项目基于原始 Asyncflow 的设计与代码结构，
通过逐章手写代码学习分布式异步任务调度。

## 工程组件

- flowsvr：任务流服务
- worker：任务执行进程
- taskutils：共享 RPC 协议和数据结构
- internal/configloader：通用 TOML 配置加载
- sql：数据库建表脚本
- script：Redis 等辅助脚本

## 第一章：配置系统

本章实现：
- 项目目录结构
- Go Module
- 通用配置加载器
- flowsvr 配置
- Worker 配置
- 配置校验及单元测试

## 初始化本地配置

cp flowsvr/src/config/config.example.toml \
   flowsvr/src/config/config.toml

cp worker/src/config/config.example.toml \
   worker/src/config/config.toml

注意：修改本地数据库和 Redis 密码，
不要提交真实凭据。

## 测试

go test ./...

## 启动配置测试入口

cd flowsvr/src
go run main.go

cd ../../worker/src
go run main.go

当前只验证配置加载。
MySQL、Redis 和任务调度将在后续章节实现。
