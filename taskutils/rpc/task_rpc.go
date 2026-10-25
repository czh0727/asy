package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TaskRpc struct {
	Host string
}

// requestJSON 承担原项目 sendRequest 的基础职责。
// 本次使用标准库实现传输层，HTTP 路径和 JSON 协议不变。
func (p *TaskRpc) requestJSON(
	method string,
	path string,
	requestBody interface{},
	response interface{},
) error {
	if strings.TrimSpace(p.Host) == "" {
		return fmt.Errorf("empty flowsvr host")
	}

	var body io.Reader

	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(
		method,
		strings.TrimRight(p.Host, "/")+path,
		body,
	)
	if err != nil {
		return err
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err := TaskClient.Do(req)
	if err != nil {
		return fmt.Errorf("flowsvr request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"flowsvr http status: %d",
			resp.StatusCode,
		)
	}

	err = json.NewDecoder(
		io.LimitReader(resp.Body, 4<<20),
	).Decode(response)
	if err != nil {
		return fmt.Errorf(
			"decode flowsvr response: %w", err,
		)
	}

	return nil
}

// GetTaskScheduleCfgList 获取任务调度配置。
func (p *TaskRpc) GetTaskScheduleCfgList() (
	*model.GetTaskScheduleCfgListResp,
	error,
) {
	result := new(model.GetTaskScheduleCfgListResp)

	err := p.requestJSON(
		http.MethodGet,
		model.GET_TASK_SCHEDULE_CFG_SUFFIX,
		nil,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return result, fmt.Errorf(
			"flowsvr schedule cfg code=%d: %s",
			result.Code, result.Msg,
		)
	}

	return result, nil
}

// HoldTasks 从 flowsvr 一次领取一批任务。
func (p *TaskRpc) HoldTasks(
	request *model.HoldTasksReq,
) (*model.HoldTasksResp, error) {
	if request == nil {
		return nil, fmt.Errorf("nil hold request")
	}

	result := new(model.HoldTasksResp)

	err := p.requestJSON(
		http.MethodPost,
		model.HOLD_TASKS,
		request,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return result, fmt.Errorf(
			"flowsvr hold code=%d: %s",
			result.Code, result.Msg,
		)
	}

	return result, nil
}

// SetTask 将业务任务的状态、阶段、上下文等
// 持久化到 flowsvr。
func (p *TaskRpc) SetTask(
	request *model.SetTaskReq,
) (*model.SetTaskResp, error) {
	if request == nil {
		return nil, fmt.Errorf("nil set task request")
	}

	result := new(model.SetTaskResp)

	err := p.requestJSON(
		http.MethodPost,
		model.SET_TASK_SUFFIX,
		request,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return result, fmt.Errorf(
			"flowsvr set task code=%d: %s",
			result.Code,
			result.Msg,
		)
	}

	return result, nil
}

// CreateTask 通过 flowsvr 创建任务。
func (p *TaskRpc) CreateTask(
	request *model.CreateTaskReq,
) (*model.CreateTaskResp, error) {
	if request == nil {
		return nil, fmt.Errorf("nil create task request")
	}

	result := new(model.CreateTaskResp)

	err := p.requestJSON(
		http.MethodPost,
		model.CREATE_TASK_SUFFIX,
		request,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return result, fmt.Errorf(
			"flowsvr create task code=%d: %s",
			result.Code,
			result.Msg,
		)
	}

	return result, nil
}

// GetTask 按 TaskId 查询任务。
func (p *TaskRpc) GetTask(
	request *model.GetTaskReq,
) (*model.GetTaskResp, error) {
	if request == nil || request.TaskId == "" {
		return nil, fmt.Errorf("task id is required")
	}

	result := new(model.GetTaskResp)

	path := model.GET_TASK_SUFFIX +
		"?task_id=" + url.QueryEscape(request.TaskId)

	err := p.requestJSON(
		http.MethodGet,
		path,
		nil,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return result, fmt.Errorf(
			"flowsvr get task code=%d: %s",
			result.Code,
			result.Msg,
		)
	}

	if result.TaskData == nil {
		return result, fmt.Errorf(
			"flowsvr returned empty task data",
		)
	}

	return result, nil
}
