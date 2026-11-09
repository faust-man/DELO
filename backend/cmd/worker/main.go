package main

import (
	"context"
	"delo/internal/app"
	"log"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	a, err := app.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer a.DB.Close()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err = app.ProcessOutbox(ctx, a.DB)
			if err != nil && ctx.Err() == nil {
				log.Println("outbox processing failed")
			}
		}
	}
}
