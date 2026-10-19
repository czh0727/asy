
# 第三章：flowsvr HTTP 控制面

## 背景问题

业务系统不能都直接操作任务数据库。

需要独立服务统一负责：
- 创建任务
- 查询任务
- 领取任务
- 完成任务
- 状态管理

## 架构设计

Client -> flowsvr -> Repository -> MySQL

Worker -> flowsvr -> Repository -> MySQL

HTTP 层不直接编写 SQL。

## 方案选择

使用 Gin 提供 HTTP API。

使用 Store 接口隔离 HTTP 与 MySQL。

使用统一错误码区分：
- 400 参数错误
- 401 内部接口未授权
- 404 任务不存在
- 409 状态冲突
- 500 系统内部错误

## 核心问题与解决方法

1. HTTP 层不直接操作数据库：
   通过 Repository 封装数据库逻辑。

2. 数据库错误不直接暴露客户端：
   记录服务端日志，返回通用错误。

3. Worker 接口不能任意调用：
   本地阶段使用 X-Worker-Token。

4. 用户输入可能异常：
   限制请求体大小，校验 JSON 与参数。

5. HTTP 处理不能无限等待：
   设置服务读写超时和数据库请求超时。

6. 服务退出不能突然中断连接：
   使用信号监听和 Shutdown 优雅退出。

## 当前局限

- 没有实现生产级业务身份和多租户权限
- Worker Token 只是教学阶段的简化鉴权
- 未实现任务执行者身份校验
- 未实现业务 Handler
- 未实现失败重试、超时恢复与任务心跳
- 未实现任务批量领取及分表

## 下一章

实现 Worker 和 TaskSDK。

由 Worker：
- 注册 Task Handler
- 轮询 flowsvr
- 领取任务
- 执行业务代码
- 回写任务结果
