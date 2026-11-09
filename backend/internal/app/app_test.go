package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRolesDenyByDefault(t *testing.T) {
	for _, role := range []string{"teacher", "analyst", "operator", "artist", "client", "", "OWNER"} {
		for name, r := range resources {
			if allowed(role, r.Write) {
				t.Fatalf("%s can write %s", role, name)
			}
		}
	}
}
func TestIntegerAmounts(t *testing.T) {
	for _, f := range []float64{0, 0.5, -1, 1.1} {
		v := number(map[string]any{"amount": f}, "amount")
		if v > 0 {
			t.Fatalf("accepted %v", f)
		}
	}
}
func TestIDs(t *testing.T) {
	seen := map[string]bool{}
	for range 10000 {
		id := ID()
		if len(id) != 36 || seen[id] {
			t.Fatal(id)
		}
		seen[id] = true
	}
}
func TestOrigin(t *testing.T) {
	a := &App{Origin: "http://localhost:8088"}
	for _, origin := range []string{"", "https://evil.test", "http://localhost:8088.evil.test"} {
		r := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(`{}`))
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
}
func TestIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("TEST_DATABASE_URL required in CI")
		}
		t.Skip("requires isolated PostgreSQL TEST_DATABASE_URL")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := "test_" + strings.ReplaceAll(ID(), "-", "")
	if _, err = db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	files, _ := filepath.Glob("../../db/migrations/*.sql")
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	org, other, uid, person, class := ID(), ID(), ID(), ID(), ID()
	pw, _ := bcrypt.GenerateFromPassword([]byte("test-password-long"), bcrypt.MinCost)
	for _, q := range []struct {
		sql  string
		args []any
	}{{"INSERT INTO organizations VALUES($1,'A'),($2,'B')", []any{org, other}}, {"INSERT INTO users VALUES($1,'test@example.local',$2)", []any{uid, string(pw)}}, {"INSERT INTO memberships(org,user_id,role) VALUES($1,$2,'owner')", []any{org, uid}}, {"INSERT INTO sessions VALUES($1,$2,now()+interval '1 hour')", []any{hash("test-token"), uid}}, {"INSERT INTO persons VALUES($1,$2,'Test','')", []any{person, org}}, {"INSERT INTO classes VALUES($1,$2,'Last seat',now()+interval '1 day',1,'scheduled')", []any{class, org}}} {
		if _, err = pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{DB: pool, Origin: "http://test.local"}
	handler := a.Handler()
	request := func(method, path, tenant, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/"+path, strings.NewReader(body))
		r.Header.Set("Origin", a.Origin)
		r.Header.Set("X-Organization-ID", tenant)
		r.Header.Set("Idempotency-Key", key)
		r.AddCookie(&http.Cookie{Name: "delo_session", Value: "test-token"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	t.Run("tenant isolation", func(t *testing.T) {
		for name := range resources {
			if w := request("GET", name, other, "", ""); w.Code != 403 {
				t.Fatal(name, w.Code)
			}
		}
		w := request("POST", "persons", other, "tenant-test", `{"name":"Intrusion"}`)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	})
	t.Run("last seat concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		codes := make(chan int, 20)
		for i := range 20 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes <- request("POST", "book", org, fmt.Sprintf("booking-%d", i), fmt.Sprintf(`{"person_id":%q,"class_id":%q}`, person, class)).Code
			}(i)
		}
		wg.Wait()
		close(codes)
		success := 0
		for code := range codes {
			if code == 200 {
				success++
			} else if code != 409 {
				t.Fatal(code)
			}
		}
		if success != 1 {
			t.Fatal(success)
		}
	})
	t.Run("idempotency", func(t *testing.T) {
		w := request("POST", "persons", org, "same-key-123", `{"name":"Once"}`)
		repeat := request("POST", "persons", org, "same-key-123", `{"name":"Once"}`)
		if w.Code != 200 || w.Body.String() != repeat.Body.String() {
			t.Fatal(w.Body.String(), repeat.Body.String())
		}
		if request("POST", "persons", org, "same-key-123", `{"name":"Changed"}`).Code != 409 {
			t.Fatal("changed body accepted")
		}
	})
	t.Run("foreign child blocked", func(t *testing.T) {
		w := request("POST", "payment", org, "foreign-person", fmt.Sprintf(`{"person_id":%q,"amount":100,"currency":"RUB","reason":"test"}`, ID()))
		if w.Code != 409 {
			t.Fatal(w.Code)
		}
	})
	t.Run("parallel refunds", func(t *testing.T) {
		w := request("POST", "payment", org, "payment-123", fmt.Sprintf(`{"person_id":%q,"amount":100,"currency":"RUB","reason":"test"}`, person))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var p map[string]string
		json.Unmarshal(w.Body.Bytes(), &p)
		var wg sync.WaitGroup
		codes := make(chan int, 10)
		for i := range 10 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes <- request("POST", "refund", org, fmt.Sprintf("refund-%d", i), fmt.Sprintf(`{"payment_id":%q,"amount":100,"reason":"test"}`, p["id"])).Code
			}(i)
		}
		wg.Wait()
		close(codes)
		n := 0
		for code := range codes {
			if code == 200 {
				n++
			}
		}
		if n != 1 {
			t.Fatal(n)
		}
	})
	t.Run("roles revalidated", func(t *testing.T) {
		pool.Exec(ctx, "UPDATE memberships SET role='teacher' WHERE org=$1", org)
		for name := range resources {
			if request("GET", name, org, "", "").Code != 403 {
				t.Fatal(name)
			}
		}
		if request("POST", "payment", org, "teacher-pay", `{}`).Code != 409 {
			t.Fatal("teacher mutation")
		}
		pool.Exec(ctx, "UPDATE memberships SET active=false WHERE org=$1", org)
		if request("GET", "persons", org, "", "").Code != 403 {
			t.Fatal("blocked session works")
		}
		pool.Exec(ctx, "UPDATE memberships SET active=true,role='owner' WHERE org=$1", org)
	})
	t.Run("atomic audit outbox", func(t *testing.T) {
		var audits, events int
		pool.QueryRow(ctx, "SELECT count(*) FROM audit").Scan(&audits)
		pool.QueryRow(ctx, "SELECT count(*) FROM outbox").Scan(&events)
		if audits != events || audits < 4 {
			t.Fatal(audits, events)
		}
	})
	t.Run("policy unknown", func(t *testing.T) {
		for _, path := range []string{"cancel-booking", "dispatch"} {
			w := request("POST", path, org, "policy-"+path, `{}`)
			if w.Code != 409 || !strings.Contains(w.Body.String(), "POLICY_REQUIRED") {
				t.Fatal(w.Body.String())
			}
		}
	})

	t.Run("subscription last visit", func(t *testing.T) {
		tariff, sub, c1, c2 := ID(), ID(), ID(), ID()
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"INSERT INTO tariffs VALUES($1,$2,'One visit',1,1500,'RUB')", []any{tariff, org}},
			{"INSERT INTO subscriptions VALUES($1,$2,$3,$4,1,1500,'RUB',now()+interval '10 days')", []any{sub, org, person, tariff}},
			{"INSERT INTO classes VALUES($1,$2,'A',now()+interval '1 day',10,'scheduled'),($3,$2,'B',now()+interval '2 days',10,'scheduled')", []any{c1, org, c2}},
		} {
			if _, err = pool.Exec(ctx, q.sql, q.args...); err != nil {
				t.Fatal(err)
			}
		}
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for i, c := range []string{c1, c2} {
			wg.Add(1)
			go func(i int, c string) {
				defer wg.Done()
				codes <- request("POST", "book", org, fmt.Sprintf("visit-last-%d", i), fmt.Sprintf(`{"class_id":%q,"person_id":%q,"subscription_id":%q}`, c, person, sub)).Code
			}(i, c)
		}
		wg.Wait()
		close(codes)
		n := 0
		for code := range codes {
			if code == 200 {
				n++
			}
		}
		if n != 1 {
			t.Fatal(n)
		}
		var count int
		pool.QueryRow(ctx, "SELECT count(*) FROM bookings WHERE org=$1 AND class_id IN ($2,$3)", org, c1, c2).Scan(&count)
		if count != 1 {
			t.Fatal("failed reservation did not rollback", count)
		}
	})
	t.Run("artwork custody", func(t *testing.T) {
		art := ID()
		pool.Exec(ctx, "INSERT INTO artworks VALUES($1,$2,'INV-1','Painting','Author','Owner','Studio','Good',false)", art, org)
		body := fmt.Sprintf(`{"artwork_id":%q,"destination":"Exhibition","reason":"Test contract"}`, art)
		if request("POST", "dispatch-artwork", org, "art-no-rights", body).Code != 409 {
			t.Fatal("no rights dispatch")
		}
		if w := request("POST", "confirm-rights", org, "rights-confirm", fmt.Sprintf(`{"artwork_id":%q,"evidence":"Synthetic contract #1"}`, art)); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		w := request("POST", "dispatch-artwork", org, "art-dispatch", body)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var movement map[string]string
		json.Unmarshal(w.Body.Bytes(), &movement)
		if request("POST", "dispatch-artwork", org, "art-dispatch-twice", body).Code != 409 {
			t.Fatal("double dispatch")
		}
		receive := fmt.Sprintf(`{"movement_id":%q,"condition":"Good"}`, movement["id"])
		if w = request("POST", "receive-artwork", org, "art-receive", receive); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if request("POST", "receive-artwork", org, "art-receive-twice", receive).Code != 409 {
			t.Fatal("double receipt")
		}
		var loc, owner string
		pool.QueryRow(ctx, "SELECT location,owner_name FROM artworks WHERE org=$1 AND id=$2", org, art).Scan(&loc, &owner)
		if loc != "Exhibition" || owner != "Owner" {
			t.Fatal(loc, owner)
		}
	})
	t.Run("payment allocation", func(t *testing.T) {
		payment, charge := ID(), ID()
		pool.Exec(ctx, "INSERT INTO payments(id,org,person_id,amount,currency,reason) VALUES($1,$2,$3,200,'RUB','test')", payment, org, person)
		pool.Exec(ctx, "INSERT INTO charges(id,org,person_id,amount,currency,reason) VALUES($1,$2,$3,100,'RUB','test')", charge, org, person)
		body := fmt.Sprintf(`{"payment_id":%q,"charge_id":%q,"amount":100}`, payment, charge)
		if w := request("POST", "allocate", org, "allocation-one", body); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if request("POST", "allocate", org, "allocation-two", body).Code != 409 {
			t.Fatal("over allocation")
		}
		if request("POST", "refund", org, "allocated-refund", fmt.Sprintf(`{"payment_id":%q,"amount":101,"reason":"test"}`, payment)).Code != 409 {
			t.Fatal("allocated refund")
		}
	})

	t.Run("worker replay and concurrent delivery", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- ProcessOutbox(ctx, pool) }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := ProcessOutbox(ctx, pool); err != nil {
			t.Fatal(err)
		}
		var events, notices, pending int
		pool.QueryRow(ctx, "SELECT count(*) FROM outbox").Scan(&events)
		pool.QueryRow(ctx, "SELECT count(*) FROM notifications").Scan(&notices)
		pool.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE delivered_at IS NULL").Scan(&pending)
		if events != notices || pending != 0 {
			t.Fatal(events, notices, pending)
		}
	})

	t.Run("explicit demo cancellation", func(t *testing.T) {
		pool.Exec(ctx, "INSERT INTO organization_policies(org,version,demo,cancel_hours) VALUES($1,1,true,24)", org)
		booking, c := ID(), ID()
		pool.Exec(ctx, "INSERT INTO classes VALUES($1,$2,'Cancel test',now()+interval '3 days',10,'scheduled')", c, org)
		pool.Exec(ctx, "INSERT INTO bookings(id,org,class_id,person_id) VALUES($1,$2,$3,$4)", booking, org, c, person)
		body := fmt.Sprintf(`{"booking_id":%q,"reason":"Demo cancellation"}`, booking)
		if w := request("POST", "cancel-booking", org, "demo-cancel-one", body); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if request("POST", "cancel-booking", org, "demo-cancel-two", body).Code != 409 {
			t.Fatal("double cancellation")
		}
		var version int
		var late bool
		pool.QueryRow(ctx, "SELECT policy_version,late FROM cancellation_events WHERE org=$1 AND booking_id=$2", org, booking).Scan(&version, &late)
		if version != 1 || late {
			t.Fatal(version, late)
		}
	})

	t.Run("schedule room conflict and adjacent interval", func(t *testing.T) {
		branch, room := ID(), ID()
		pool.Exec(ctx, "INSERT INTO branches(id,org,name,address,timezone) VALUES($1,$2,'Branch','Synthetic address','Europe/Moscow')", branch, org)
		pool.Exec(ctx, "INSERT INTO rooms VALUES($1,$2,$3,'Room',5)", room, org, branch)
		start := time.Now().Add(10 * 24 * time.Hour).UTC().Truncate(time.Hour)
		command := func(key string, offset time.Duration, capacity int) int {
			body := fmt.Sprintf(`{"name":"Lesson","starts":%q,"ends":%q,"capacity":%d,"room_id":%q}`, start.Add(offset).Format(time.RFC3339), start.Add(offset+time.Hour).Format(time.RFC3339), capacity, room)
			return request("POST", "classes", org, key, body).Code
		}
		if c := command("room-first", 0, 5); c != 200 {
			t.Fatal(c)
		}
		if c := command("room-overlap", 30*time.Minute, 5); c != 409 {
			t.Fatal(c)
		}
		if c := command("room-adjacent", time.Hour, 5); c != 200 {
			t.Fatal(c)
		}
		if c := command("room-capacity", 3*time.Hour, 6); c != 409 {
			t.Fatal(c)
		}
	})
	t.Run("login session cookie and logout", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(`{"login":"test@example.local","password":"test-password-long"}`))
		r.Header.Set("Origin", a.Origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
			t.Fatal("cookie protections")
		}
		r = httptest.NewRequest("GET", "/api/v1/me", nil)
		r.AddCookie(cookies[0])
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), org) {
			t.Fatal(w.Body.String())
		}
		r = httptest.NewRequest("POST", "/api/v1/logout", strings.NewReader(`{}`))
		r.Header.Set("Origin", a.Origin)
		r.AddCookie(cookies[0])
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		r = httptest.NewRequest("GET", "/api/v1/me", nil)
		r.AddCookie(cookies[0])
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("logout failed")
		}
	})

	t.Run("numeric input and permission replay", func(t *testing.T) {
		for i, amount := range []string{`1.5`, `1e30`, `"100"`, `-1`} {
			w := request("POST", "payment", org, fmt.Sprintf("bad-amount-%d", i), fmt.Sprintf(`{"person_id":%q,"amount":%s,"currency":"RUB","reason":"test"}`, person, amount))
			if w.Code != 400 {
				t.Fatal(amount, w.Code)
			}
		}
		pool.Exec(ctx, "UPDATE memberships SET role='teacher' WHERE org=$1", org)
		if request("POST", "persons", org, "same-key-123", `{"name":"Once"}`).Code != 409 {
			t.Fatal("cached response bypasses current permissions")
		}
		pool.Exec(ctx, "UPDATE memberships SET role='owner' WHERE org=$1", org)
	})
	t.Run("session expiry", func(t *testing.T) {
		pool.Exec(ctx, "UPDATE sessions SET expires=$1", time.Now().Add(-time.Hour))
		if request("GET", "persons", org, "", "").Code != 401 {
			t.Fatal("expired session works")
		}
	})
}
