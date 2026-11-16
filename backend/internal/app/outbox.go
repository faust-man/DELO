package app

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProcessOutbox(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `WITH pending AS (SELECT id,org,action FROM outbox WHERE delivered_at IS NULL ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 100), delivered AS (INSERT INTO notifications(id,org,message) SELECT id,org,action FROM pending ON CONFLICT DO NOTHING) UPDATE outbox SET delivered_at=now() WHERE id IN (SELECT id FROM pending)`)
	return err
}
