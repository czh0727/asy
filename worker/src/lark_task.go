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
        fmt.Println("第一阶段：发送飞书消息")

        // 模拟发送消息成功
        p.TaskStage = "record"

        // 保存业务上下文
        p.SetContextLocal(p.ContextData)

        // 注意：不设置 success
        // run() 会自动将 processing 改成 pending
        return nil

    case "record":
        fmt.Println("第二阶段：记录消息发送结果")

        // 整个任务完成
        p.Status = int(constant.TASK_STATUS_SUCC)

        return nil

    default:
        p.Status = int(constant.TASK_STATUS_FAILED)
        return fmt.Errorf(
            "unknown task stage: %s",
            p.TaskStage,
        )
    }
}


// HandleFinish 任务完成后的收尾函数。
func (p *LarkTask) HandleFinish() {
	// 原项目这里暂时是空实现。
}
