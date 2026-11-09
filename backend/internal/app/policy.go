package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

func cancelBooking(ctx context.Context, tx pgx.Tx, org, role, id string, v map[string]any) error {
	if !allowed(role, []string{"owner", "admin"}) {
		return fmt.Errorf("Недостаточно прав")
	}
	var version, hours int
	var demo bool
	if tx.QueryRow(ctx, "SELECT version,cancel_hours,demo FROM organization_policies WHERE org=$1 ORDER BY version DESC LIMIT 1", org).Scan(&version, &hours, &demo) != nil || !demo {
		return fmt.Errorf("POLICY_REQUIRED: правила отмены не согласованы; разрешён только явно включённый demo-профиль")
	}
	var starts time.Time
	var state string
	if tx.QueryRow(ctx, "SELECT c.starts,b.state FROM bookings b JOIN classes c ON c.org=b.org AND c.id=b.class_id WHERE b.org=$1 AND b.id=$2 FOR UPDATE OF b", org, str(v, "booking_id")).Scan(&starts, &state) != nil {
		return fmt.Errorf("Запись не найдена")
	}
	if state != "booked" || str(v, "reason") == "" {
		return fmt.Errorf("Нужны активная запись и основание")
	}
	late := time.Now().After(starts.Add(-time.Duration(hours) * time.Hour))
	newState := "released"
	if late {
		newState = "consumed"
	}
	if _, err := tx.Exec(ctx, "INSERT INTO cancellation_events(id,org,booking_id,policy_version,late,reason) VALUES($1,$2,$3,$4,$5,$6)", id, org, str(v, "booking_id"), version, late, str(v, "reason")); err != nil {
		return fmt.Errorf("Отмена уже проведена")
	}
	if _, err := tx.Exec(ctx, "UPDATE visit_reservations SET state=$3 WHERE org=$1 AND booking_id=$2 AND state='reserved'", org, str(v, "booking_id"), newState); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "UPDATE bookings SET state='cancelled' WHERE org=$1 AND id=$2", org, str(v, "booking_id"))
	return err
}
func init() {
	resources["organization_policies"] = Resource{"version AS id,version,demo,cancel_hours,created_at", []string{"owner", "admin"}, nil}
	resources["cancellation_events"] = Resource{"id,booking_id,policy_version,late,reason,created_at", []string{"owner", "admin"}, nil}
}
