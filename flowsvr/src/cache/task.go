package cache

import (
	"encoding/json"
	"errors"

	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/redis/go-redis/v9"
)

func CreateTask(task *db.Task) error {
	if task == nil {
		return errors.New("nil task")
	}
	if rdb == nil {
		return errors.New("redis not initialized")
	}

	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	key := prefix + task.TaskId
	return rdb.Set(key, string(data), expireTime)
}

func FindTaskByTaskId(
	taskID string,
) (*db.Task, error) {
	if rdb == nil {
		return nil, errors.New("redis not initialized")
	}

	key := prefix + taskID

	value, err := rdb.Get(key)
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}

	var task db.Task
	if err := json.Unmarshal(
		[]byte(value), &task,
	); err != nil {
		return nil, err
	}

	return &task, nil
}

func OnceDeleteTask(taskID string) {
	if rdb == nil {
		return
	}

	key := prefix + taskID
	_ = rdb.Delete(key)
}
