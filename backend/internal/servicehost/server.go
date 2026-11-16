// Package servicehost starts a single module. Its identity is fixed by its entrypoint.
package servicehost

import (
	"context"
	"delo/internal/app"
	"delo/internal/topology"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func Run(module string) {
	if _, ok := topology.Modules[module]; !ok {
		log.Fatal("unknown service")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	a, err := app.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer a.DB.Close()
	Serve(ctx, topology.Scope(module, a.Handler()))
}

func Serve(ctx context.Context, handler http.Handler) {
	server := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Println("HTTP listening :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
