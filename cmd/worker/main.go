
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/czh0727/asy/internal/tasksdk"
)

func main() {
	addr := os.Getenv("FLOWSVR_URL")
	if addr == "" {
		addr = "http://127.0.0.1:41555"
	}

	client, err := tasksdk.NewHTTPClient(
		addr,
		os.Getenv("WORKER_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 创建 Handler 注册中心
	registry := tasksdk.NewRegistry()

	// 注册 video.generate 任务
	err = registry.Register(
		"video.generate",
		tasksdk.HandlerFunc(
			func(
				ctx context.Context,
				raw json.RawMessage,
			) error {
				var req struct {
					Prompt string `json:"prompt"`
				}

				if err := json.Unmarshal(
					raw, &req,
				); err != nil {
					return err
				}

				if req.Prompt == "" {
					return errors.New(
						"prompt is required",
					)
				}

				log.Printf(
					"simulate video generation: prompt=%q",
					req.Prompt,
				)

				// 模拟耗时两秒的视频生成
				timer := time.NewTimer(
					2 * time.Second,
				)
				defer timer.Stop()

				select {
				case <-ctx.Done():
					return ctx.Err()

				case <-timer.C:
					log.Printf(
						"simulated video generated: prompt=%q",
						req.Prompt,
					)
					return nil
				}
			},
		),
	)

	if err != nil {
		log.Fatal(err)
	}

	// 创建 Worker
	worker, err := tasksdk.NewWorker(
		client,
		registry,
		"video.generate",
		time.Second,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 支持 Ctrl+C 和 SIGTERM
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Print("worker started: type=video.generate")

	if err := worker.Run(ctx); err != nil {
		log.Fatal(err)
	}

	log.Print("worker stopped")
}
