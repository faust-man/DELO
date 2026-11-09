package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

func init() {
	resources["rights_confirmations"] = Resource{"id,artwork_id,evidence,created_at", []string{"owner", "curator"}, nil}
	resources["artwork_receipts"] = Resource{"id,movement_id,condition,created_at", []string{"owner", "curator"}, nil}
	for name, res := range map[string]Resource{
		"relationships":     {"id,representative_id,child_id,reason", []string{"owner", "admin"}, nil},
		"tariffs":           {"id,name,visits,price,currency", []string{"owner", "admin", "cashier"}, nil},
		"subscriptions":     {"id,person_id,tariff_id,visits,price,currency,expires", []string{"owner", "admin", "cashier"}, nil},
		"waitlist":          {"id,class_id,person_id,created_at", []string{"owner", "admin"}, nil},
		"attendance_events": {"id,booking_id,status,reason,created_at", []string{"owner", "admin"}, nil},
		"charges":           {"id,person_id,amount,currency,reason,created_at", []string{"owner", "cashier"}, nil},
		"allocations":       {"id,payment_id,charge_id,amount", []string{"owner", "cashier"}, nil},
	} {
		resources[name] = res
	}
}
func workflow(ctx context.Context, tx pgx.Tx, org, role, path, id string, v map[string]any) error {
	exec := func(q string, args ...any) error {
		_, err := tx.Exec(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("Нарушены ограничения данных или связей")
		}
		return nil
	}
	if !allowed(role, []string{"owner", "admin", "cashier", "curator"}) {
		return fmt.Errorf("Недостаточно прав")
	}
	switch path {
	case "relationships":
		if !allowed(role, []string{"owner", "admin"}) {
			break
		}
		if str(v, "reason") == "" {
			return fmt.Errorf("Укажите основание связи")
		}
		return exec("INSERT INTO relationships VALUES($1,$2,$3,$4,$5)", id, org, str(v, "representative_id"), str(v, "child_id"), str(v, "reason"))
	case "tariffs":
		if role != "owner" {
			break
		}
		if str(v, "name") == "" {
			return fmt.Errorf("Укажите название")
		}
		return exec("INSERT INTO tariffs VALUES($1,$2,$3,$4,$5,$6)", id, org, str(v, "name"), number(v, "visits"), number(v, "price"), str(v, "currency"))
	case "subscriptions":
		if !allowed(role, []string{"owner", "admin"}) {
			break
		}
		expires, err := time.Parse(time.RFC3339, str(v, "expires"))
		if err != nil || !expires.After(time.Now()) {
			return fmt.Errorf("Укажите будущий срок действия")
		}
		tag, err := tx.Exec(ctx, "INSERT INTO subscriptions SELECT $1,org,$3,id,visits,price,currency,$4::timestamptz FROM tariffs WHERE org=$2 AND id=$5", id, org, str(v, "person_id"), expires, str(v, "tariff_id"))
		if err != nil || tag.RowsAffected() != 1 {
			return fmt.Errorf("Тариф или клиент не найден")
		}
		return nil
	case "waitlist":
		if !allowed(role, []string{"owner", "admin"}) {
			break
		}
		return exec("INSERT INTO waitlist(id,org,class_id,person_id) VALUES($1,$2,$3,$4)", id, org, str(v, "class_id"), str(v, "person_id"))
	case "attendance":
		if !allowed(role, []string{"owner", "admin"}) {
			break
		}
		if str(v, "status") != "present" {
			return fmt.Errorf("POLICY_REQUIRED: списания пропуска и коррекции не согласованы")
		}
		var state string
		var starts time.Time
		if tx.QueryRow(ctx, "SELECT b.state,c.starts FROM bookings b JOIN classes c ON c.org=b.org AND c.id=b.class_id WHERE b.org=$1 AND b.id=$2", org, str(v, "booking_id")).Scan(&state, &starts) != nil {
			return fmt.Errorf("Запись не найдена")
		}
		if starts.After(time.Now()) || state != "booked" {
			return fmt.Errorf("Отметка доступна для состоявшегося занятия и активной записи")
		}
		if err := exec("INSERT INTO attendance_events(id,org,booking_id,status,reason) VALUES($1,$2,$3,'present',$4)", id, org, str(v, "booking_id"), str(v, "reason")); err != nil {
			return err
		}
		return exec("UPDATE visit_reservations SET state='consumed' WHERE org=$1 AND booking_id=$2 AND state='reserved'", org, str(v, "booking_id"))
	case "charges":
		if !allowed(role, []string{"owner", "cashier"}) {
			break
		}
		if str(v, "reason") == "" {
			return fmt.Errorf("Основание обязательно")
		}
		return exec("INSERT INTO charges(id,org,person_id,amount,currency,reason) VALUES($1,$2,$3,$4,$5,$6)", id, org, str(v, "person_id"), number(v, "amount"), str(v, "currency"), str(v, "reason"))
	case "allocate":
		if !allowed(role, []string{"owner", "cashier"}) {
			break
		}
		var available, due int64
		var pc, cc, pp, cp string
		if tx.QueryRow(ctx, `SELECT p.amount-coalesce((SELECT sum(amount) FROM allocations WHERE org=p.org AND payment_id=p.id),0)-coalesce((SELECT sum(amount) FROM refunds WHERE org=p.org AND payment_id=p.id),0),p.currency,p.person_id FROM payments p WHERE org=$1 AND id=$2`, org, str(v, "payment_id")).Scan(&available, &pc, &pp) != nil {
			return fmt.Errorf("Платёж не найден")
		}
		if tx.QueryRow(ctx, `SELECT c.amount-coalesce((SELECT sum(amount) FROM allocations WHERE org=c.org AND charge_id=c.id),0),c.currency,c.person_id FROM charges c WHERE org=$1 AND id=$2`, org, str(v, "charge_id")).Scan(&due, &cc, &cp) != nil {
			return fmt.Errorf("Начисление не найдено")
		}
		amount := number(v, "amount")
		if amount <= 0 || amount > available || amount > due || pc != cc || pp != cp {
			return fmt.Errorf("Недопустимое распределение: сумма, валюта или плательщик")
		}
		return exec("INSERT INTO allocations VALUES($1,$2,$3,$4,$5)", id, org, str(v, "payment_id"), str(v, "charge_id"), amount)
	case "confirm-rights":
		if !allowed(role, []string{"owner", "curator"}) {
			break
		}
		if str(v, "evidence") == "" {
			return fmt.Errorf("Укажите документальное основание экспонирования")
		}
		if err := exec("INSERT INTO rights_confirmations(id,org,artwork_id,evidence) VALUES($1,$2,$3,$4)", id, org, str(v, "artwork_id"), str(v, "evidence")); err != nil {
			return err
		}
		return exec("UPDATE artworks SET rights_confirmed=true WHERE org=$1 AND id=$2", org, str(v, "artwork_id"))
	case "dispatch-artwork":
		if !allowed(role, []string{"owner", "curator"}) {
			break
		}
		var location string
		var rights bool
		if tx.QueryRow(ctx, "SELECT location,rights_confirmed FROM artworks WHERE org=$1 AND id=$2 FOR UPDATE", org, str(v, "artwork_id")).Scan(&location, &rights) != nil {
			return fmt.Errorf("Работа не найдена")
		}
		if !rights {
			return fmt.Errorf("Права на экспонирование не подтверждены")
		}
		if str(v, "destination") == "" || str(v, "reason") == "" || str(v, "destination") == location {
			return fmt.Errorf("Укажите другое место назначения и основание")
		}
		if err := exec("INSERT INTO movements(id,org,artwork_id,source,destination,state,reason) VALUES($1,$2,$3,$4,$5,'dispatched',$6)", id, org, str(v, "artwork_id"), location, str(v, "destination"), str(v, "reason")); err != nil {
			return err
		}
		return exec("UPDATE artworks SET location='В пути' WHERE org=$1 AND id=$2", org, str(v, "artwork_id"))
	case "receive-artwork":
		if !allowed(role, []string{"owner", "curator"}) {
			break
		}
		var art, dest string
		if tx.QueryRow(ctx, "SELECT artwork_id,destination FROM movements WHERE org=$1 AND id=$2 AND state='dispatched' FOR UPDATE", org, str(v, "movement_id")).Scan(&art, &dest) != nil {
			return fmt.Errorf("Отправка не найдена или уже принята")
		}
		if str(v, "condition") == "" {
			return fmt.Errorf("Укажите состояние при приёме")
		}
		if err := exec("INSERT INTO artwork_receipts(id,org,movement_id,condition) VALUES($1,$2,$3,$4)", id, org, str(v, "movement_id"), str(v, "condition")); err != nil {
			return err
		}
		if err := exec("UPDATE artworks SET location=$3,condition=$4 WHERE org=$1 AND id=$2", org, art, dest, str(v, "condition")); err != nil {
			return err
		}
		return exec("UPDATE movements SET state='received' WHERE org=$1 AND id=$2", org, str(v, "movement_id"))
	}
	return scheduling(ctx, tx, org, role, path, id, v)
}
func reserveVisit(ctx context.Context, tx pgx.Tx, org, booking, subscription, person, class string) error {
	var available int
	var owner string
	var expiry, start time.Time
	err := tx.QueryRow(ctx, `SELECT s.person_id,s.expires,s.visits-(SELECT count(*) FROM visit_reservations r WHERE r.org=s.org AND r.subscription_id=s.id AND r.state IN ('reserved','consumed')) FROM subscriptions s WHERE s.org=$1 AND s.id=$2 FOR UPDATE`, org, subscription).Scan(&owner, &expiry, &available)
	if err != nil {
		return fmt.Errorf("Абонемент не найден")
	}
	if err = tx.QueryRow(ctx, "SELECT starts FROM classes WHERE org=$1 AND id=$2", org, class).Scan(&start); err != nil {
		return err
	}
	if available < 1 || owner != person || !start.Before(expiry) {
		return fmt.Errorf("Абонемент исчерпан, чужой или истекает до занятия")
	}
	_, err = tx.Exec(ctx, "INSERT INTO visit_reservations VALUES($1,$2,$3,'reserved')", org, booking, subscription)
	return err
}
