package task

import "time"

const (
	RetryBaseDelay = 2 * time.Second
	RetryMaxDelay  = time.Minute
)

// RetryResult 是一次可重试失败处理后的持久化结果。
type RetryResult struct {
	Status     Status    `json:"status"`
	RetryCount int       `json:"retry_count"`
	MaxRetries int       `json:"max_retries"`
	OrderTime  time.Time `json:"order_time"`
}

// RetryDelay 根据当前 retry_count 计算下一次退避时间。
//
// retry_count = 0: 2s
// retry_count = 1: 4s
// retry_count = 2: 8s
// ...
// 最大延迟 60s。
func RetryDelay(retryCount int) time.Duration {
	delay := RetryBaseDelay

	for i := 0; i < retryCount && delay < RetryMaxDelay; i++ {
		// 避免不断倍增带来的溢出风险。
		if delay >= RetryMaxDelay/2 {
			return RetryMaxDelay
		}
		delay *= 2
	}

	if delay > RetryMaxDelay {
		return RetryMaxDelay
	}
	return delay
}
