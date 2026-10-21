package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
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

	registry := tasksdk.NewRegistry()

	// 注册模拟视频生成 Handler。
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

				// 模拟耗时两秒的业务操作。
				select {
				case <-ctx.Done():
					return ctx.Err()

				case <-time.After(2 * time.Second):
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

	// 默认最大并发数 = 3
	concurrency := 3

	if s := os.Getenv("WORKER_CONCURRENCY"); s != "" {
		concurrency, err = strconv.Atoi(s)
		if err != nil {
			log.Fatalf(
				"invalid WORKER_CONCURRENCY: %v", err,
			)
		}
	}

	// 默认优雅退出宽限期 = 10 秒
	grace := 10 * time.Second

	if s := os.Getenv("WORKER_SHUTDOWN_GRACE"); s != "" {
		grace, err = time.ParseDuration(s)
		if err != nil {
			log.Fatalf(
				"invalid WORKER_SHUTDOWN_GRACE: %v", err,
			)
		}
	}

	worker, err := tasksdk.NewWorkerWithOptions(
		client,
		registry,
		"video.generate",
		time.Second,
		tasksdk.WorkerOptions{
			MaxConcurrent: concurrency,
			ShutdownGrace: grace,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Printf(
		"worker started: type=video.generate concurrency=%d grace=%v",
		concurrency,
		grace,
	)

	if err := worker.Run(ctx); err != nil {
		log.Printf(
			"worker stopped with error: %v", err,
		)
		return
	}

	log.Print("worker stopped gracefully")
}
