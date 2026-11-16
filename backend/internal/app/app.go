package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type App struct {
	DB        *pgxpool.Pool
	Origin    string
	Secure    bool
	dummyOnce sync.Once
	dummyHash string
}
type Resource struct {
	Columns string
	Read    []string
	Write   []string
}

var resources = map[string]Resource{
	"persons":       {"id,name,contact", []string{"owner", "admin"}, []string{"owner", "admin"}},
	"classes":       {"id,name,starts,capacity,state", []string{"owner", "admin"}, []string{"owner", "admin"}},
	"bookings":      {"id,class_id,person_id,state", []string{"owner", "admin"}, nil},
	"artworks":      {"id,number,name,author,owner_name,location,condition,rights_confirmed", []string{"owner", "admin", "curator"}, []string{"owner", "curator"}},
	"exhibitions":   {"id,name,starts,ends", []string{"owner", "admin", "curator"}, []string{"owner", "curator"}},
	"movements":     {"id,artwork_id,source,destination,state,reason,created_at", []string{"owner", "curator"}, nil},
	"payments":      {"id,person_id,amount,currency,reason,created_at", []string{"owner", "cashier"}, nil},
	"refunds":       {"id,payment_id,amount,reason,created_at", []string{"owner", "cashier"}, nil},
	"audit":         {"id,actor,action,object_id,created_at", []string{"owner"}, nil},
	"notifications": {"id,message,created_at", []string{"owner", "admin", "curator", "cashier"}, nil},
}

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func allowed(role string, roles []string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, s string) {
	reply(w, status, map[string]string{"error": s})
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if a.DB.Ping(ctx) != nil {
			fail(w, 503, "database unavailable")
			return
		}
		reply(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/v1/login", a.login)
	mux.HandleFunc("/api/v1/", a.api)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Request-ID", ID())
		if r.Method != "GET" && r.Header.Get("Origin") != a.Origin {
			fail(w, 403, "Недопустимый Origin")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		mux.ServeHTTP(w, r)
	})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var v struct{ Login, Password string }
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		fail(w, 400, "Некорректный JSON")
		return
	}
	if len(v.Login) > 254 || len(v.Password) > 72 {
		fail(w, 401, "Неверные данные входа")
		return
	}
	var attempts int
	err := a.DB.QueryRow(r.Context(), `INSERT INTO login_limits(key,attempts,until_at) VALUES($1,1,now()+interval '15 minutes') ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN login_limits.until_at<now() THEN 1 ELSE login_limits.attempts+1 END,until_at=CASE WHEN login_limits.until_at<now() THEN now()+interval '15 minutes' ELSE login_limits.until_at END RETURNING attempts`, hash(strings.ToLower(v.Login))).Scan(&attempts)
	if err != nil {
		fail(w, 503, "Вход временно недоступен")
		return
	}
	if attempts > 20 {
		fail(w, 429, "Повторите позже")
		return
	}
	var uid, pw string
	err = a.DB.QueryRow(r.Context(), "SELECT id,password_hash FROM users WHERE login=$1", strings.ToLower(v.Login)).Scan(&uid, &pw)
	a.dummyOnce.Do(func() { p, _ := bcrypt.GenerateFromPassword([]byte(ID()), 12); a.dummyHash = string(p) })
	if err != nil {
		pw = a.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword([]byte(pw), []byte(v.Password))
	if err != nil || passwordErr != nil {
		fail(w, 401, "Неверные данные входа")
		return
	}
	token := ID() + ID()
	_, err = a.DB.Exec(r.Context(), "INSERT INTO sessions VALUES($1,$2,now()+interval '8 hours')", hash(token), uid)
	if err != nil {
		fail(w, 500, "Ошибка сессии")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "delo_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	reply(w, 200, map[string]string{"status": "ok"})
}
func (a *App) api(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	c, err := r.Cookie("delo_session")
	if err != nil {
		fail(w, 401, "Войдите в систему")
		return
	}
	var uid string
	if a.DB.QueryRow(ctx, "SELECT user_id FROM sessions WHERE token=$1 AND expires>now()", hash(c.Value)).Scan(&uid) != nil {
		fail(w, 401, "Сессия истекла")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if path == "logout" && r.Method == "POST" {
		a.DB.Exec(ctx, "DELETE FROM sessions WHERE token=$1", hash(c.Value))
		http.SetCookie(w, &http.Cookie{Name: "delo_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode})
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	if path == "me" && r.Method == "GET" {
		rows, err := a.DB.Query(ctx, "SELECT o.id,o.name,m.role FROM memberships m JOIN organizations o ON o.id=m.org WHERE m.user_id=$1 AND m.active", uid)
		if err != nil {
			fail(w, 500, "Ошибка БД")
			return
		}
		defer rows.Close()
		out := []map[string]string{}
		for rows.Next() {
			var id, name, role string
			rows.Scan(&id, &name, &role)
			out = append(out, map[string]string{"id": id, "name": name, "role": role})
		}
		reply(w, 200, out)
		return
	}
	org := r.Header.Get("X-Organization-ID")
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		fail(w, 503, "БД недоступна")
		return
	}
	defer tx.Rollback(ctx)
	// Serialize all mutations for an organization, including membership revalidation.
	if r.Method != "GET" {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org); err != nil {
			fail(w, 500, "Блокировка недоступна")
			return
		}
	}
	var role string
	if tx.QueryRow(ctx, "SELECT role FROM memberships WHERE org=$1 AND user_id=$2 AND active", org, uid).Scan(&role) != nil {
		fail(w, 403, "Нет доступа к организации")
		return
	}
	res, ok := resources[path]
	if r.Method == "GET" && ok {
		if !allowed(role, res.Read) {
			fail(w, 403, "Недостаточно прав")
			return
		}
		rows, err := tx.Query(ctx, "SELECT row_to_json(t) FROM (SELECT "+res.Columns+" FROM "+path+" WHERE org=$1 ORDER BY id LIMIT 500) t", org)
		if err != nil {
			fail(w, 500, "Ошибка чтения")
			return
		}
		out := []json.RawMessage{}
		for rows.Next() {
			var b []byte
			rows.Scan(&b)
			out = append(out, b)
		}
		rows.Close()
		reply(w, 200, out)
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Метод недоступен")
		return
	}
	var v map[string]any
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		fail(w, 400, "Некорректный JSON")
		return
	}
	for _, field := range []string{"amount", "price", "capacity", "visits"} {
		if value, exists := v[field]; exists {
			f, ok := value.(float64)
			if !ok || f < 0 || f > 9007199254740991 || f != float64(int64(f)) {
				fail(w, 400, "Числовые поля должны быть целыми в безопасном диапазоне")
				return
			}
		}
	}
	if !commandAllowed(role, path) {
		fail(w, 409, "Недостаточно прав")
		return
	}
	b, _ := json.Marshal(v)
	fingerprint := hash(path + string(b))
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		fail(w, 400, "Нужен Idempotency-Key длиной 8–128")
		return
	}
	var oldHash string
	var old []byte
	err = tx.QueryRow(ctx, "SELECT request_hash,response FROM idempotency WHERE org=$1 AND key=$2", org, key).Scan(&oldHash, &old)
	if err == nil {
		if oldHash != fingerprint {
			fail(w, 409, "Ключ уже использован с другим запросом")
			return
		}
		reply(w, 200, json.RawMessage(old))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		fail(w, 500, "Ошибка идемпотентности")
		return
	}
	id := ID()
	err = execute(ctx, tx, org, role, path, id, v)
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	var audit int64
	err = tx.QueryRow(ctx, "INSERT INTO audit(org,actor,action,object_id) VALUES($1,$2,$3,$4) RETURNING id", org, uid, path, id).Scan(&audit)
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO outbox(id,org,action) VALUES($1,$2,$3)", audit, org, path)
	}
	result := map[string]string{"id": id}
	encoded, _ := json.Marshal(result)
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO idempotency VALUES($1,$2,$3,$4)", org, key, fingerprint, encoded)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		fail(w, 500, "Транзакция не завершена")
		return
	}
	reply(w, 200, result)
}
func str(v map[string]any, k string) string { s, _ := v[k].(string); return strings.TrimSpace(s) }
func number(v map[string]any, k string) int64 {
	f, _ := v[k].(float64)
	if f != float64(int64(f)) {
		return 0
	}
	return int64(f)
}
func execute(ctx context.Context, tx pgx.Tx, org, role, path, id string, v map[string]any) error {
	exec := func(q string, args ...any) error {
		_, err := tx.Exec(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("Данные нарушают ограничения: проверьте поля и связи")
		}
		return nil
	}
	if res, ok := resources[path]; ok && res.Write != nil {
		if !allowed(role, res.Write) {
			return fmt.Errorf("Недостаточно прав")
		}
		if str(v, "name") == "" {
			return fmt.Errorf("Название обязательно")
		}
		switch path {
		case "persons":
			return exec("INSERT INTO persons VALUES($1,$2,$3,$4)", id, org, str(v, "name"), str(v, "contact"))
		case "classes":
			return createClass(ctx, tx, org, id, v)
		case "artworks":
			for _, k := range []string{"number", "author", "owner_name", "location", "condition"} {
				if str(v, k) == "" {
					return fmt.Errorf("Обязательное поле: %s", k)
				}
			}
			return exec("INSERT INTO artworks VALUES($1,$2,$3,$4,$5,$6,$7,$8,false)", id, org, str(v, "number"), str(v, "name"), str(v, "author"), str(v, "owner_name"), str(v, "location"), str(v, "condition"))
		case "exhibitions":
			return exec("INSERT INTO exhibitions VALUES($1,$2,$3,$4,$5)", id, org, str(v, "name"), str(v, "starts"), str(v, "ends"))
		}
	}
	switch path {
	case "book":
		if !allowed(role, []string{"owner", "admin"}) {
			break
		}
		var capacity, count int
		var state string
		var starts time.Time
		if tx.QueryRow(ctx, "SELECT capacity,state,starts FROM classes WHERE org=$1 AND id=$2 FOR UPDATE", org, str(v, "class_id")).Scan(&capacity, &state, &starts) != nil {
			return fmt.Errorf("Занятие не найдено")
		}
		if state != "scheduled" || starts.Before(time.Now()) {
			return fmt.Errorf("Занятие недоступно")
		}
		tx.QueryRow(ctx, "SELECT count(*) FROM bookings WHERE org=$1 AND class_id=$2 AND state<>'cancelled'", org, str(v, "class_id")).Scan(&count)
		if count >= capacity {
			return fmt.Errorf("Нет свободных мест")
		}
		if err := exec("INSERT INTO bookings(id,org,class_id,person_id) VALUES($1,$2,$3,$4)", id, org, str(v, "class_id"), str(v, "person_id")); err != nil {
			return err
		}
		if str(v, "subscription_id") != "" {
			return reserveVisit(ctx, tx, org, id, str(v, "subscription_id"), str(v, "person_id"), str(v, "class_id"))
		}
		return nil
	case "cancel-booking":
		return cancelBooking(ctx, tx, org, role, id, v)
	case "payment":
		if !allowed(role, []string{"owner", "cashier"}) {
			break
		}
		if str(v, "reason") == "" {
			return fmt.Errorf("Основание обязательно")
		}
		return exec("INSERT INTO payments(id,org,person_id,amount,currency,reason) VALUES($1,$2,$3,$4,$5,$6)", id, org, str(v, "person_id"), number(v, "amount"), str(v, "currency"), str(v, "reason"))
	case "refund":
		if !allowed(role, []string{"owner", "cashier"}) {
			break
		}
		var total, refunded int64
		if tx.QueryRow(ctx, "SELECT amount FROM payments WHERE org=$1 AND id=$2 FOR UPDATE", org, str(v, "payment_id")).Scan(&total) != nil {
			return fmt.Errorf("Платёж не найден")
		}
		tx.QueryRow(ctx, "SELECT coalesce(sum(amount),0) FROM refunds WHERE org=$1 AND payment_id=$2", org, str(v, "payment_id")).Scan(&refunded)
		var allocated int64
		if err := tx.QueryRow(ctx, "SELECT coalesce(sum(amount),0) FROM allocations WHERE org=$1 AND payment_id=$2", org, str(v, "payment_id")).Scan(&allocated); err != nil {
			return fmt.Errorf("Ошибка распределений")
		}
		refunded += allocated
		amount := number(v, "amount")
		if amount <= 0 || amount > total-refunded {
			return fmt.Errorf("Возврат превышает остаток")
		}
		if str(v, "reason") == "" {
			return fmt.Errorf("Основание обязательно")
		}
		return exec("INSERT INTO refunds(id,org,payment_id,amount,reason) VALUES($1,$2,$3,$4,$5)", id, org, str(v, "payment_id"), amount, str(v, "reason"))
	case "dispatch":
		return fmt.Errorf("POLICY_REQUIRED: документы передачи и права на экспонирование не согласованы")
	default:
		return workflow(ctx, tx, org, role, path, id, v)
	}
	return fmt.Errorf("Недостаточно прав")
}
func Open(ctx context.Context) (*App, error) {
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}
	// Bound the total database footprint when every module has its own pool.
	config.MaxConns = 4
	config.MinConns = 0
	db, err := pgxpool.NewWithConfig(ctx, config)
	return &App{DB: db, Origin: os.Getenv("APP_ORIGIN"), Secure: os.Getenv("COOKIE_SECURE") != "false"}, err
}
func Bootstrap(ctx context.Context, db *pgxpool.Pool) error {
	password := os.Getenv("BOOTSTRAP_PASSWORD")
	if len(password) < 12 {
		return fmt.Errorf("BOOTSTRAP_PASSWORD: минимум 12 символов")
	}
	pw, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	uid, org := ID(), ID()
	if _, err = tx.Exec(ctx, "INSERT INTO users VALUES($1,$2,$3)", uid, "owner@demo.local", string(pw)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO organizations VALUES($1,'ДЕЛО • тестовая студия')", org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO memberships(org,user_id,role) VALUES($1,$2,'owner')", org, uid); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO organization_policies(org,version,demo,cancel_hours) VALUES($1,1,true,24)", org); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
