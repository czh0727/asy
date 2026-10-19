
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/czh0727/asy/internal/storage/mysql"
	"github.com/czh0727/asy/internal/task"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN is required (add parseTime=true&loc=UTC)")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	repo := mysql.New(db)

	t, err := task.NewTask(
		"demo.video",
		json.RawMessage(`{"prompt":"a cat"}`),
		10,
		time.Now().Add(-time.Minute),
		3,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 使用专属任务类型，避免领取到旧的演示任务。
	t.Type = "demo.video." + t.ID

	if err := repo.Create(ctx, t); err != nil {
		log.Fatal(err)
	}
	fmt.Println("created:", t.ID, t.Status)

	claimed, err := repo.ClaimNext(ctx, t.Type)
	if err != nil {
		if errors.Is(err, mysql.ErrNoReadyTask) {
			log.Fatal("there is no ready task")
		}
		log.Fatal(err)
	}
	fmt.Println("claimed:", claimed.ID, claimed.Status)

	if err := repo.Finish(
		ctx, claimed.ID, task.Success,
	); err != nil {
		log.Fatal(err)
	}

	stored, err := repo.Get(ctx, claimed.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("finished:", stored.ID, stored.Status)
}
