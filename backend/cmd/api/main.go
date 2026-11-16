package main

import (
	"context"
	"delo/internal/app"
	"delo/internal/topology"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	module := os.Getenv("SERVICE_NAME")
	var handler http.Handler
	if module == "gateway" {
		handler = topology.Gateway()
	} else {
		if module != "" {
			if _, ok := topology.Modules[module]; !ok {
				log.Fatal("unknown SERVICE_NAME")
			}
		}
		a, err := app.Open(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer a.DB.Close()
		handler = a.Handler()
		if module != "" {
			handler = topology.Scope(module, handler)
		}
	}
	s := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.Shutdown(c)
	}()
	log.Println("API listening :8080")
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
