
# 第一章：项目骨架与配置系统

## 1. 学习目标

从零重建 Asyncflow 项目骨架。

以原项目 czh0727/asyn 为实现依据，
保留 flowsvr、worker、taskutils、
internal/configloader 等模块划分。

## 2. 配置加载过程

每个组件拥有自己的 main.go。

程序加载 config 包时，通过 init()
向上查找 main.go，定位组件根目录。

随后调用 config.Init()，
读取根目录下的 config/config.toml。

使用 BurntSushi/toml 将配置映射为 Go 结构体，
然后执行 Validate()。

## 3. 为什么分为两个配置模块？

flowsvr 负责数据库、Redis、HTTP 和任务治理。

worker 负责执行任务，需要知道
flowsvr 地址与 Redis 锁配置。

两者配置项不同，但可以共用配置加载器。

## 4. 当前状态

已实现配置加载及测试。

暂未连接 MySQL、Redis。
暂未启动 HTTP 服务器。
暂未实现 TaskSDK。

## 5. 与原项目的差异

Go Module 改成 github.com/czh0727/asy。

Go 版本使用较新基线。

可提交的示例配置使用 config.example.toml，
本地实际配置 config.toml 不提交。

程序入口暂时只执行配置验证，
后续逐章补齐原项目初始化流程。

## 6. 下一章

实现原项目 MySQL、Redis 初始化，
并建立原始任务配置与调度位置相关数据表。
