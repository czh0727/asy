
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/czh0727/asy/internal/httpapi"
	"github.com/czh0727/asy/internal/storage/mysql"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	token := os.Getenv("WORKER_TOKEN")

	if dsn == "" || token == "" {
		log.Fatal("MYSQL_DSN and WORKER_TOKEN must be set")
	}

	// 初始化 MySQL 连接池
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	// 启动前检查数据库
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	if err := db.PingContext(ctx); err != nil {
		cancel()
		log.Fatal(err)
	}
	cancel()

	addr := os.Getenv("FLOWSVR_ADDR")
	if addr == "" {
		addr = "127.0.0.1:41555"
	}

	// 注入 Repository
	handler := httpapi.NewHandler(
		mysql.New(db),
		token,
		db.PingContext,
	)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 监听 Ctrl+C 和终止信号
	root, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		log.Printf("flowsvr listening on %s", addr)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
			stop()
		}
	}()

	// 等待服务停止
	<-root.Done()

	// 优雅退出，最多等待 8 秒
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		8*time.Second,
	)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
