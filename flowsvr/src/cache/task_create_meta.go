package cache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/jinzhu/gorm"
)

const (
	createMetaCacheTTL = 5 * time.Second
	createMetaTimeout  = 100 * time.Millisecond
)

type TaskCreateMeta struct {
	EndPos        int
	MaxRetryNum   int
	RetryInterval int
}

var ErrTaskConfig = errors.New("get task config")

func taskCreateMetaKey(taskType string) string {
	return "asyncflow:create_meta:" + taskType
}

// Redis Lua：只允许较新的写入位置覆盖旧缓存。
const updateCreateMetaScript = `
local current = tonumber(redis.call('HGET', KEYS[1], 'end_pos'))

if not current or current < tonumber(ARGV[1]) then
    redis.call('HSET', KEYS[1], 'end_pos', ARGV[1])
end

if #ARGV == 4 then
    redis.call(
        'HSET',
        KEYS[1],
        'max_retry_num',
        ARGV[3],
        'retry_interval',
        ARGV[4]
    )
end

redis.call('EXPIRE', KEYS[1], ARGV[2])

return 1
`

// GetTaskCreateMeta 获取创建任务所需的元数据。
// Redis 不可用时回退到 MySQL。
func GetTaskCreateMeta(
	database *gorm.DB,
	taskType string,
) (TaskCreateMeta, error) {

	if database == nil {
		return TaskCreateMeta{}, errors.New("database is nil")
	}

	// 1. 优先查询 Redis。
	if rdb != nil {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			createMetaTimeout,
		)

		values, err := rdb.RedisPool.
			HGetAll(ctx, taskCreateMetaKey(taskType)).
			Result()

		cancel()

		if err == nil {
			pos, posErr := strconv.Atoi(values["end_pos"])
			maxRetry, retryErr := strconv.Atoi(
				values["max_retry_num"],
			)
			retryInterval, intervalErr := strconv.Atoi(
				values["retry_interval"],
			)

			if posErr == nil &&
				retryErr == nil &&
				intervalErr == nil &&
				pos > 0 {

				return TaskCreateMeta{
					EndPos:        pos,
					MaxRetryNum:   maxRetry,
					RetryInterval: retryInterval,
				}, nil
			}
		}
	}

	// 2. Redis 没有数据，回源 MySQL。
	taskPos, err := db.TaskPosNsp.GetTaskPos(
		database,
		taskType,
	)
	if err != nil {
		return TaskCreateMeta{}, err
	}

	cfg, err := db.TaskTypeCfgNsp.GetTaskTypeCfg(
		database,
		taskType,
	)
	if err != nil {
		return TaskCreateMeta{},
			fmt.Errorf("%w: %v", ErrTaskConfig, err)
	}

	meta := TaskCreateMeta{
		EndPos:        taskPos.ScheduleEndPos,
		MaxRetryNum:   cfg.MaxRetryNum,
		RetryInterval: cfg.RetryInterval,
	}

	// 3. 回填 Redis。
	if rdb != nil {
		if err := setTaskCreateMeta(taskType, meta); err != nil {
			log.Printf("fill task meta cache failed: %v", err)
		}
	}

	return meta, nil
}

func setTaskCreateMeta(
	taskType string,
	meta TaskCreateMeta,
) error {

	if rdb == nil {
		return errors.New("redis is not initialized")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		createMetaTimeout,
	)
	defer cancel()

	return rdb.RedisPool.Eval(
		ctx,
		updateCreateMetaScript,
		[]string{taskCreateMetaKey(taskType)},
		meta.EndPos,
		int(createMetaCacheTTL/time.Second),
		meta.MaxRetryNum,
		meta.RetryInterval,
	).Err()
}

// SetTaskWritePos 更新任务写入位置缓存。
func SetTaskWritePos(
	taskType string,
	pos int,
) error {

	if rdb == nil {
		return errors.New("redis is not initialized")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		createMetaTimeout,
	)
	defer cancel()

	return rdb.RedisPool.Eval(
		ctx,
		updateCreateMetaScript,
		[]string{taskCreateMetaKey(taskType)},
		pos,
		int(createMetaCacheTTL/time.Second),
	).Err()
}
