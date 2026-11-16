package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// BootstrapDemoAdmin is an explicit, temporary local-development seed.
// Fixed IDs keep retries isolated from existing studios and accounts.
func BootstrapDemoAdmin(ctx context.Context, db *pgxpool.Pool) error {
	const uid = "00000000-0000-4000-8000-000000000001"
	const org = "00000000-0000-4000-8000-000000000002"
	pw, err := bcrypt.GenerateFromPassword([]byte("admin"), 12)
	if err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users(id,login,password_hash) VALUES($1,'admin@admin.com',$2) ON CONFLICT(id) DO UPDATE SET login=EXCLUDED.login,password_hash=EXCLUDED.password_hash WHERE users.login IN ('admin','admin@admin.com')", []any{uid, string(pw)}},
		{"INSERT INTO organizations(id,name) VALUES($1,'ДЕЛО • демо-студия') ON CONFLICT DO NOTHING", []any{org}},
		{"INSERT INTO memberships(org,user_id,role) VALUES($1,$2,'owner') ON CONFLICT(org,user_id) DO UPDATE SET role='owner',active=true", []any{org, uid}},
		{"INSERT INTO organization_policies(org,version,demo,cancel_hours) VALUES($1,1,true,24) ON CONFLICT DO NOTHING", []any{org}},
	} {
		if _, err = tx.Exec(ctx, q.sql, q.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
