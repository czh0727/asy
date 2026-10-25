package db

type TaskEnum int

const (
	TASK_STATUS_PENDING    TaskEnum = 1
	TASK_STATUS_PROCESSING TaskEnum = 2
	TASK_STATUS_SUCCESS    TaskEnum = 3
	TASK_STATUS_FAILED     TaskEnum = 4
)

// 原项目定义的最大优先级，约一年的秒数。
const MAX_PRIORITY = 3600 * 24 * 30 * 12

func IsValidStatus(status TaskEnum) bool {
	switch status {
	case TASK_STATUS_PENDING,
		TASK_STATUS_PROCESSING,
		TASK_STATUS_SUCCESS,
		TASK_STATUS_FAILED:
		return true
	default:
		return false
	}
}
