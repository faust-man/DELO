package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

func init() {
	resources["branches"] = Resource{"id,name,address,timezone,active", []string{"owner", "admin", "curator"}, nil}
	resources["rooms"] = Resource{"id,branch_id,name,capacity", []string{"owner", "admin"}, nil}
	resources["groups"] = Resource{"id,name", []string{"owner", "admin"}, nil}
	resources["class_changes"] = Resource{"id,class_id,old_starts,old_ends,new_starts,new_ends,reason,created_at", []string{"owner", "admin"}, nil}
	resources["operational_tasks"] = Resource{"id,object_id,reason,state,created_at", []string{"owner", "admin", "curator", "cashier"}, nil}
	r := resources["classes"]
	r.Columns = "id,name,starts,ends,capacity,state,room_id,teacher_id,group_id"
	resources["classes"] = r
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func validateInterval(ctx context.Context, tx pgx.Tx, org, id, room, teacher string, start, end time.Time, capacity int64) error {
	if !end.After(start) || capacity < 1 {
		return fmt.Errorf("Нужны положительная длительность и вместимость")
	}
	if room != "" {
		var limit int64
		var active bool
		if tx.QueryRow(ctx, "SELECT r.capacity,b.active FROM rooms r JOIN branches b ON b.org=r.org AND b.id=r.branch_id WHERE r.org=$1 AND r.id=$2", org, room).Scan(&limit, &active) != nil || !active || capacity > limit {
			return fmt.Errorf("Помещение закрыто, не найдено или вместимость превышена")
		}
	}
	var conflict bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM classes WHERE org=$1 AND id<>$2 AND state<>'cancelled' AND starts<$4 AND ends>$3 AND ((room_id=$5::uuid AND $5::uuid IS NOT NULL) OR (teacher_id=$6::uuid AND $6::uuid IS NOT NULL)))`, org, id, start, end, nullable(room), nullable(teacher)).Scan(&conflict)
	if err != nil {
		return fmt.Errorf("Не удалось проверить расписание")
	}
	if conflict {
		return fmt.Errorf("Пересечение помещения или преподавателя")
	}
	return nil
}
func createClass(ctx context.Context, tx pgx.Tx, org, id string, v map[string]any) error {
	start, err := time.Parse(time.RFC3339, str(v, "starts"))
	if err != nil {
		return fmt.Errorf("Начало должно содержать UTC offset")
	}
	end, err := time.Parse(time.RFC3339, str(v, "ends"))
	if err != nil {
		return fmt.Errorf("Окончание должно содержать UTC offset")
	}
	if !start.After(time.Now()) {
		return fmt.Errorf("Создавайте будущие занятия")
	}
	if err = validateInterval(ctx, tx, org, id, str(v, "room_id"), str(v, "teacher_id"), start, end, number(v, "capacity")); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO classes(id,org,name,starts,ends,capacity,room_id,teacher_id,group_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", id, org, str(v, "name"), start, end, number(v, "capacity"), nullable(str(v, "room_id")), nullable(str(v, "teacher_id")), nullable(str(v, "group_id")))
	if err != nil {
		return fmt.Errorf("Некорректные связи занятия")
	}
	return nil
}
func scheduling(ctx context.Context, tx pgx.Tx, org, role, path, id string, v map[string]any) error {
	if !allowed(role, []string{"owner", "admin"}) {
		return fmt.Errorf("Недостаточно прав")
	}
	switch path {
	case "branches":
		if _, err := time.LoadLocation(str(v, "timezone")); err != nil {
			return fmt.Errorf("Укажите часовой пояс IANA")
		}
		if str(v, "name") == "" || str(v, "address") == "" {
			return fmt.Errorf("Название и адрес обязательны")
		}
		_, err := tx.Exec(ctx, "INSERT INTO branches(id,org,name,address,timezone) VALUES($1,$2,$3,$4,$5)", id, org, str(v, "name"), str(v, "address"), str(v, "timezone"))
		return err
	case "rooms":
		if str(v, "name") == "" {
			return fmt.Errorf("Название обязательно")
		}
		_, err := tx.Exec(ctx, "INSERT INTO rooms VALUES($1,$2,$3,$4,$5)", id, org, str(v, "branch_id"), str(v, "name"), number(v, "capacity"))
		if err != nil {
			return fmt.Errorf("Проверьте филиал и вместимость")
		}
		return nil
	case "groups":
		if str(v, "name") == "" {
			return fmt.Errorf("Название обязательно")
		}
		_, err := tx.Exec(ctx, "INSERT INTO groups VALUES($1,$2,$3)", id, org, str(v, "name"))
		return err
	case "move-class", "cancel-class", "complete-class":
		var start time.Time
		var end *time.Time
		var room, teacher *string
		var capacity int64
		var state string
		if tx.QueryRow(ctx, "SELECT starts,ends,room_id,teacher_id,capacity,state FROM classes WHERE org=$1 AND id=$2 FOR UPDATE", org, str(v, "class_id")).Scan(&start, &end, &room, &teacher, &capacity, &state) != nil {
			return fmt.Errorf("Занятие не найдено")
		}
		if state != "scheduled" {
			return fmt.Errorf("Занятие уже завершено или отменено")
		}
		if str(v, "reason") == "" {
			return fmt.Errorf("Основание обязательно")
		}
		if path == "complete-class" {
			if start.After(time.Now()) {
				return fmt.Errorf("Занятие ещё не началось")
			}
			var unmarked int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM bookings b WHERE org=$1 AND class_id=$2 AND state='booked' AND NOT EXISTS(SELECT 1 FROM attendance_events e WHERE e.org=b.org AND e.booking_id=b.id)", org, str(v, "class_id")).Scan(&unmarked); err != nil {
				return err
			}
			if unmarked > 0 && v["confirm_unmarked"] != true {
				return fmt.Errorf("Есть неотмеченные ученики; нужно явное подтверждение")
			}
			_, err := tx.Exec(ctx, "UPDATE classes SET state='completed' WHERE org=$1 AND id=$2", org, str(v, "class_id"))
			return err
		}
		if !start.After(time.Now()) {
			return fmt.Errorf("Прошедшее занятие нельзя автоматически переносить/отменять")
		}
		if path == "move-class" {
			ns, err := time.Parse(time.RFC3339, str(v, "starts"))
			if err != nil {
				return fmt.Errorf("Некорректное начало")
			}
			ne, err := time.Parse(time.RFC3339, str(v, "ends"))
			if err != nil || !ns.After(time.Now()) {
				return fmt.Errorf("Некорректное окончание или прошедшая дата")
			}
			r, t := "", ""
			if room != nil {
				r = *room
			}
			if teacher != nil {
				t = *teacher
			}
			if err = validateInterval(ctx, tx, org, str(v, "class_id"), r, t, ns, ne, capacity); err != nil {
				return err
			}
			var incompatible bool
			if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM bookings b JOIN visit_reservations r ON r.org=b.org AND r.booking_id=b.id JOIN subscriptions s ON s.org=r.org AND s.id=r.subscription_id WHERE b.org=$1 AND b.class_id=$2 AND r.state='reserved' AND s.expires<=$3)", org, str(v, "class_id"), ns).Scan(&incompatible); err != nil {
				return err
			}
			if incompatible {
				return fmt.Errorf("После переноса истечёт абонемент; сначала разрешите резерв")
			}
			if _, err = tx.Exec(ctx, "INSERT INTO class_changes(id,org,class_id,old_starts,old_ends,new_starts,new_ends,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", id, org, str(v, "class_id"), start, end, ns, ne, str(v, "reason")); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "UPDATE classes SET starts=$3,ends=$4 WHERE org=$1 AND id=$2", org, str(v, "class_id"), ns, ne)
			return err
		}
		// Studio cancellation releases rights and creates a visible task; no automatic refund.
		if _, err := tx.Exec(ctx, "UPDATE visit_reservations r SET state='released' FROM bookings b WHERE b.org=r.org AND b.id=r.booking_id AND b.org=$1 AND b.class_id=$2 AND r.state='reserved'", org, str(v, "class_id")); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE bookings SET state='cancelled' WHERE org=$1 AND class_id=$2 AND state='booked'", org, str(v, "class_id")); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO operational_tasks(id,org,object_id,reason) VALUES($1,$2,$3,$4)", id, org, str(v, "class_id"), "Отмена студией: согласовать компенсации и расчёты. "+str(v, "reason")); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE classes SET state='cancelled' WHERE org=$1 AND id=$2", org, str(v, "class_id"))
		return err
	}
	return fmt.Errorf("Операция не реализована")
}
