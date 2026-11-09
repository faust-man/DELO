package main

import (
	"context"
	"delo/internal/app"
	"log"
	"os"
	"path/filepath"
)

func main() {
	ctx := context.Background()
	a, err := app.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer a.DB.Close()
	if len(os.Args) < 2 {
		log.Fatal("migrate | bootstrap")
	}
	switch os.Args[1] {
	case "migrate":
		conn, err := a.DB.Acquire(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Release()
		if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(16090001)"); err != nil {
			log.Fatal(err)
		}
		defer conn.Exec(ctx, "SELECT pg_advisory_unlock(16090001)")
		files, err := filepath.Glob("db/migrations/*.sql")
		if err != nil {
			log.Fatal(err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
			log.Fatal(err)
		}
		for _, file := range files {
			var done bool
			if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", filepath.Base(file)).Scan(&done); err != nil {
				log.Fatal(err)
			}
			if done {
				continue
			}
			sql, err := os.ReadFile(file)
			if err != nil {
				log.Fatal(err)
			}
			if _, err = tx.Exec(ctx, string(sql)); err != nil {
				log.Fatal(err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(name) VALUES($1)", filepath.Base(file)); err != nil {
				log.Fatal(err)
			}
		}
		if err = tx.Commit(ctx); err != nil {
			log.Fatal(err)
		}
	case "bootstrap":
		if err = app.Bootstrap(ctx, a.DB); err != nil {
			log.Fatal(err)
		}
	case "demo-policy":
		if os.Getenv("ENABLE_DEMO_POLICY") != "true" {
			log.Fatal("ENABLE_DEMO_POLICY=true required")
		}
		if _, err = a.DB.Exec(ctx, "INSERT INTO organization_policies(org,version,demo,cancel_hours) SELECT id,1,true,24 FROM organizations ON CONFLICT DO NOTHING"); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown command")
	}
}
