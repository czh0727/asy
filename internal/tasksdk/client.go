
package tasksdk

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czh0727/asy/internal/task"
)

// ClaimedTask 是 flowsvr 返回的任务执行数据。
type ClaimedTask struct {
	ID       string          `json:"id"`
	TaskType string          `json:"task_type"`
	Status   task.Status     `json:"status"`
	Payload  json.RawMessage `json:"payload"`
}

// Worker 依赖的网络能力。
type TaskClient interface {
	Claim(ctx context.Context, taskType string) (*ClaimedTask, error)
	Finish(ctx context.Context, id string, status task.Status) error
}

type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewHTTPClient(baseURL, token string) (*HTTPClient, error) {
	u, err := url.Parse(baseURL)

	if err != nil ||
		u.Host == "" ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		u.User != nil ||
		u.RawQuery != "" ||
		u.Fragment != "" ||
		(u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid flowsvr base URL")
	}

	if token == "" {
		return nil, errors.New("worker token is empty")
	}

	return &HTTPClient{
		baseURL: strings.TrimRight(u.String(), "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

// post 统一发送 Worker 内部 HTTP 请求。
func (c *HTTPClient) post(
	ctx context.Context,
	path string,
	data any,
) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"request flowsvr: %w", err,
		)
	}

	return resp, nil
}

func httpStatusError(
	operation string,
	resp *http.Response,
) error {
	b, _ := io.ReadAll(
		io.LimitReader(resp.Body, 2048),
	)

	return fmt.Errorf(
		"%s: flowsvr returned HTTP %d: %s",
		operation,
		resp.StatusCode,
		strings.TrimSpace(string(b)),
	)
}

// Claim 从 flowsvr 领取一条任务。
// 200: 成功；204: 当前没有任务。
func (c *HTTPClient) Claim(
	ctx context.Context,
	taskType string,
) (*ClaimedTask, error) {
	resp, err := c.post(
		ctx,
		"/v1/internal/tasks/claim",
		map[string]string{
			"task_type": taskType,
		},
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError("claim", resp)
	}

	var t ClaimedTask

	err = json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	).Decode(&t)
	if err != nil {
		return nil, fmt.Errorf(
			"decode claimed task: %w", err,
		)
	}

	if !validTaskID(t.ID) ||
		t.TaskType != taskType ||
		t.Status != task.Processing {
		return nil, errors.New(
			"invalid claimed task from flowsvr",
		)
	}

	return &t, nil
}

func validTaskID(s string) bool {
	if len(s) != 32 {
		return false
	}

	_, err := hex.DecodeString(s)
	return err == nil
}

// Finish 将执行结果回写到 flowsvr。
// 本章只支持 success 和 failed。
func (c *HTTPClient) Finish(
	ctx context.Context,
	id string,
	status task.Status,
) error {
	if !validTaskID(id) {
		return errors.New("invalid task id")
	}

	if status != task.Success &&
		status != task.Failed {
		return errors.New("invalid finish status")
	}

	resp, err := c.post(
		ctx,
		"/v1/internal/tasks/"+id+"/finish",
		map[string]task.Status{
			"status": status,
		},
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return httpStatusError("finish", resp)
	}

	return nil
}
