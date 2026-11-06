package entry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"habit-go/internal/db"
)

const testHabitID = "11111111-1111-4111-8111-111111111111"

func newEntryMux(t *testing.T) *http.ServeMux {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	_, err = database.Exec(`INSERT INTO habits (id, name, type, goal_value, goal_period, updated_at)
		VALUES (?, 'Sport', 'numerical', 3, 'daily', '2025-01-01T00:00:00Z')`, testHabitID)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(NewRepository(database))
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/habits/{id}/entries/{date}", h.Upsert)
	mux.HandleFunc("DELETE /api/habits/{id}/entries/{date}", h.Delete)
	mux.HandleFunc("GET /api/habits/{id}/entries", h.GetRange)
	mux.HandleFunc("GET /api/entries", h.Bulk)
	return mux
}

func call(mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeEntry(t *testing.T, rec *httptest.ResponseRecorder) Entry {
	t.Helper()
	var e Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return e
}

func TestUpsertCreatesThenUpdates(t *testing.T) {
	mux := newEntryMux(t)
	path := "/api/habits/" + testHabitID + "/entries/2025-01-01"

	rec := call(mux, "PUT", path, `{"value":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d", rec.Code)
	}
	if e := decodeEntry(t, rec); e.Value != 1 || e.Date != "2025-01-01" {
		t.Errorf("created = %+v", e)
	}

	rec = call(mux, "PUT", path, `{"value":3}`)
	if e := decodeEntry(t, rec); e.Value != 3 {
		t.Errorf("updated value = %v, want 3", e.Value)
	}

	all := call(mux, "GET", "/api/habits/"+testHabitID+"/entries", "")
	var list []Entry
	if err := json.Unmarshal(all.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("rows = %d, want 1 (UNIQUE habit_id,date)", len(list))
	}
}

// An entry for an unknown habit is a 404, not a 500.
func TestUpsertUnknownHabitIs404(t *testing.T) {
	mux := newEntryMux(t)
	rec := call(mux, "PUT", "/api/habits/22222222-2222-4222-8222-222222222222/entries/2025-01-01", `{"value":1}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body %q)", rec.Code, rec.Body.String())
	}
}

func TestUpsertRejectsInvalidInput(t *testing.T) {
	mux := newEntryMux(t)
	base := "/api/habits/" + testHabitID + "/entries/"
	cases := map[string]struct{ path, body string }{
		"bad date": {base + "2025-13-45", `{"value":1}`},
		"negative": {base + "2025-01-01", `{"value":-1}`},
		"bad json": {base + "2025-01-01", `{"value":`},
	}
	for name, c := range cases {
		if rec := call(mux, "PUT", c.path, c.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
	}
}

func TestDeleteEntry(t *testing.T) {
	mux := newEntryMux(t)
	path := "/api/habits/" + testHabitID + "/entries/2025-01-02"
	call(mux, "PUT", path, `{"value":2}`)
	if rec := call(mux, "DELETE", path, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec := call(mux, "GET", "/api/habits/"+testHabitID+"/entries", "")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("after delete body = %q, want []", rec.Body.String())
	}
}

func TestGetRangeFiltersAndNeverReturnsNull(t *testing.T) {
	mux := newEntryMux(t)
	base := "/api/habits/" + testHabitID + "/entries/"
	call(mux, "PUT", base+"2025-01-01", `{"value":1}`)
	call(mux, "PUT", base+"2025-01-05", `{"value":5}`)
	call(mux, "PUT", base+"2025-02-01", `{"value":9}`)

	rec := call(mux, "GET", "/api/habits/"+testHabitID+"/entries?from=2025-01-02&to=2025-01-31", "")
	var list []Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Date != "2025-01-05" {
		t.Errorf("range = %+v, want only 2025-01-05", list)
	}

	empty := call(mux, "GET", "/api/habits/"+testHabitID+"/entries?from=2030-01-01", "")
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Errorf("empty range body = %q, want []", empty.Body.String())
	}
}

func TestBulkReturnsAllHabitsInRange(t *testing.T) {
	mux := newEntryMux(t)
	base := "/api/habits/" + testHabitID + "/entries/"
	call(mux, "PUT", base+"2025-03-10", `{"value":1}`)
	call(mux, "PUT", base+"2025-04-10", `{"value":2}`)

	rec := call(mux, "GET", "/api/entries?from=2025-03-01&to=2025-03-31", "")
	var list []Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Date != "2025-03-10" {
		t.Errorf("bulk = %+v, want only 2025-03-10", list)
	}
}
