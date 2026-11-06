package habit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"habit-go/internal/db"
	"habit-go/internal/entry"
)

func newMux(t *testing.T) (*http.ServeMux, *entry.Repository) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	repo := NewRepository(database)
	entryRepo := entry.NewRepository(database)
	h := NewHandler(repo, entryRepo)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/habits", h.Create)
	mux.HandleFunc("GET /api/habits", h.List)
	mux.HandleFunc("GET /api/habits/{id}", h.Get)
	return mux, entryRepo
}

func do(mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// C11: invalid goal is a 400 with the message, not a 500.
func TestCreateInvalidGoalIs400(t *testing.T) {
	mux, _ := newMux(t)
	rec := do(mux, "POST", "/api/habits", `{"name":"A","type":"boolean","goal_value":0,"goal_period":"daily"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
	}
}

// C13: the JSON Content-Type must be sent with the 201.
func TestCreateReturns201JSON(t *testing.T) {
	mux, _ := newMux(t)
	rec := do(mux, "POST", "/api/habits", `{"name":"Sport","type":"numerical","goal_value":3,"goal_period":"weekly","unit":"km"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// A7: "zero cigarettes" is expressible: goal 0 on a negative habit is accepted.
func TestCreateNegativeGoalZeroAccepted(t *testing.T) {
	mux, _ := newMux(t)
	rec := do(mux, "POST", "/api/habits", `{"name":"Cigarette","type":"boolean","goal_value":0,"goal_period":"daily","is_negative":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
}

// B6: with_entries is bounded.
func TestWithEntriesIsBounded(t *testing.T) {
	mux, _ := newMux(t)
	for _, q := range []string{"999999999", "0", "-3", "abc"} {
		rec := do(mux, "GET", "/api/habits?with_entries="+q, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("with_entries=%s: status = %d, want 400", q, rec.Code)
		}
	}
}

// A7: a slip on a negative boolean habit with goal 0 is a failure, with a full percentage.
func TestNegativeBooleanSlipIsFailure(t *testing.T) {
	mux, entryRepo := newMux(t)
	rec := do(mux, "POST", "/api/habits", `{"name":"Cigarette","type":"boolean","goal_value":0,"goal_period":"daily","is_negative":true}`)
	var created Habit
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if _, err := entryRepo.Upsert(created.ID, "2025-06-10", 1); err != nil {
		t.Fatal(err)
	}
	rec = do(mux, "GET", "/api/habits/"+created.ID+"?today=2025-06-10", "")
	var got HabitWithEntries
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress == nil || got.Progress.Success || got.Progress.Percentage != 100 {
		t.Errorf("progress = %+v, want failure at 100%%", got.Progress)
	}
}

func TestComputeProgressGoalZeroNoSlip(t *testing.T) {
	hab := &Habit{Type: "boolean", GoalValue: 0, GoalPeriod: "daily", IsNegative: true}
	base, _ := time.Parse("2006-01-02", "2025-06-10")
	p := computeProgress(hab, nil, base)
	if !p.Success || p.Percentage != 0 {
		t.Errorf("no entry: progress = %+v, want success at 0%%", p)
	}
}

// computeProgress only counts the goal window (weekly = last 7 days).
func TestComputeProgressWindow(t *testing.T) {
	hab := &Habit{Type: "numerical", GoalValue: 10, GoalPeriod: "weekly"}
	base, _ := time.Parse("2006-01-02", "2025-06-10")
	entries := []entry.Entry{
		{Date: "2025-06-03", Value: 100}, // outside the window
		{Date: "2025-06-04", Value: 4},
		{Date: "2025-06-10", Value: 6},
		{Date: "2025-06-11", Value: 50}, // future
	}
	p := computeProgress(hab, entries, base)
	if p.Current != 10 || !p.Success {
		t.Errorf("progress = %+v, want current 10 and success", p)
	}
}
