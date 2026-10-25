package main

import (
	"encoding/json"
	"fmt"

	"github.com/czh0727/asy/taskutils/constant"
	"github.com/czh0727/asy/worker/src/tasksdk"
)

// LarkReq 表示飞书任务使用的示例请求结构。
type LarkReq struct {
	Msg      string
	FromAddr string
	ToAddr   string
}

// LarkTaskContext 保存具体业务上下文。
type LarkTaskContext struct {
	ReqBody *LarkReq
	UserId  string
}

// LarkTask 通过嵌入 TaskBase 复用 TaskSDK 的能力。
type LarkTask struct {
	tasksdk.TaskBase
	ContextData *LarkTaskContext
}

// ContextLoad 从持久化 TaskContext 恢复业务数据。
func (p *LarkTask) ContextLoad() error {
	if p.TaskContext == "" {
		return fmt.Errorf("empty lark task context")
	}

	var data LarkTaskContext
	if err := json.Unmarshal(
		[]byte(p.TaskContext),
		&data,
	); err != nil {
		return fmt.Errorf("decode lark context: %w", err)
	}

	if data.ReqBody == nil {
		data.ReqBody = new(LarkReq)
	}

	p.ContextData = &data
	return nil
}

// HandleProcess 模拟原项目的飞书业务流程。
func (p *LarkTask) HandleProcess() error {
	switch p.TaskStage {
	case "sendmsg":
		// 执行 sendmsg 后，更新业务阶段和上下文。
		p.TaskStage = "record"
		p.SetContextLocal(p.ContextData)

		// 原项目使用 fallthrough。
		// 因此同一轮会继续执行 record 分支。
		fallthrough

	case "record":
		p.TaskStage = "record"

		p.Base().Status = int(
			constant.TASK_STATUS_SUCC,
		)

		fmt.Printf(
			"处理 task 完成: %s, msg=%s\n",
			p.TaskId,
			p.ContextData.ReqBody.Msg,
		)

	default:
		p.Base().Status = int(
			constant.TASK_STATUS_FAILED,
		)
	}

	return nil
}

// HandleFinish 任务完成后的收尾函数。
func (p *LarkTask) HandleFinish() {
	// 原项目这里暂时是空实现。
}
