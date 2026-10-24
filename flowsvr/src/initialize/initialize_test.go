
package initialize

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/czh0727/asy/flowsvr/src/cache"
	"github.com/czh0727/asy/flowsvr/src/config"
	"github.com/czh0727/asy/flowsvr/src/db"
)

func TestInitResourceIntegration(t *testing.T) {
	// 默认 go test 不依赖 Docker。
	// 显式启用后，才执行真实数据库测试。
	if os.Getenv("ASY_INTEGRATION") != "1" {
		t.Skip("set ASY_INTEGRATION=1 to test MySQL and Redis")
	}

	config.Init()

	InitResource()

	t.Cleanup(func() {
		_ = cache.Close()

		if db.DB != nil {
			_ = db.DB.Close()
		}
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	// 1. 检查 MySQL 是否可用。
	if err := db.DB.DB().PingContext(ctx); err != nil {
		t.Fatalf("mysql ping: %v", err)
	}

	// 2. 检查原项目的三张基础表。
	var tableCount int

	err := db.DB.DB().QueryRowContext(
		ctx,
		`SELECT COUNT(*)
           FROM information_schema.tables
          WHERE table_schema = ?
            AND table_name IN (
                't_lark_task_1',
                't_schedule_cfg',
                't_schedule_pos'
            )`,
		config.Conf.MySQL.Dbname,
	).Scan(&tableCount)

	if err != nil {
		t.Fatalf("check tables: %v", err)
	}

	if tableCount != 3 {
		t.Fatalf(
			"table count=%d, want 3",
			tableCount,
		)
	}

	// 3. 检查原项目默认 lark 调度配置。
	var limit, retryNum int

	err = db.DB.DB().QueryRowContext(
		ctx,
		`SELECT schedule_limit, max_retry_num
           FROM t_schedule_cfg
          WHERE task_type = ?`,
		"lark",
	).Scan(&limit, &retryNum)

	if err != nil {
		t.Fatalf("query schedule config: %v", err)
	}

	if limit != 100 || retryNum != 3 {
		t.Fatalf(
			"limit=%d retry=%d, want 100 and 3",
			limit, retryNum,
		)
	}

	// 4. 检查初始调度分表位置。
	var beginPos, endPos int

	err = db.DB.DB().QueryRowContext(
		ctx,
		`SELECT schedule_begin_pos, schedule_end_pos
           FROM t_schedule_pos
          WHERE task_type = ?`,
		"lark",
	).Scan(&beginPos, &endPos)

	if err != nil {
		t.Fatalf("query schedule position: %v", err)
	}

	if beginPos != 1 || endPos != 1 {
		t.Fatalf(
			"begin=%d end=%d, want 1 and 1",
			beginPos, endPos,
		)
	}

	// 5. Redis PING。
	if err := cache.Ping(ctx); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	t.Log("MySQL, Redis and original task tables verified")
}
