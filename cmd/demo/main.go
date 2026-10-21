package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/czh0727/asy/internal/task"
)

func main() {
	t, err := task.NewTask(
		"video.generate",
		json.RawMessage(`{"prompt":"a cat"}`),
		10,
		time.Time{},
		3,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"created: id=%s status=%s runnable=%v\n",
		t.ID,
		t.Status,
		t.Runnable(time.Now()),
	)

	if err := t.Transition(task.Processing); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("claimed: status=%s\n", t.Status)

	if err := t.Transition(task.Success); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("finished: status=%s\n", t.Status)
}
