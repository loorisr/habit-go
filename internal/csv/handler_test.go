package csvhandler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"habit-go/internal/db"

	"github.com/google/uuid"
)

func setup(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return NewHandler(database), database
}

func insertHabit(t *testing.T, database *sql.DB, name, typ string, goal float64, archived bool) string {
	t.Helper()
	id := uuid.NewString()
	var arch any
	if archived {
		arch = "2025-01-01T00:00:00Z"
	}
	_, err := database.Exec(`INSERT INTO habits (id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at)
		VALUES (?, ?, 'Santé', ?, ?, 'daily', 0, 'km', ?, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		id, name, typ, goal, arch)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func post(t *testing.T, fn http.HandlerFunc, body io.Reader) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/import", body)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	fn(rec, req)
	var out map[string]any
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("bad json %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

func errCount(t *testing.T, out map[string]any) int {
	t.Helper()
	list, ok := out["errors"].([]any)
	if !ok {
		t.Fatalf("errors is not an array: %#v", out["errors"])
	}
	return len(list)
}

// A1: a persistent body read error must abort the import, not loop forever
// (which also kept the single SQLite connection locked).
type failingReader struct{ calls int }

func (f *failingReader) Read(p []byte) (int, error) {
	f.calls++
	if f.calls > 1000 { // safety net so a regression cannot hang the test
		return 0, io.EOF
	}
	return 0, errors.New("connection reset")
}

func TestImportHabitsAbortsOnBodyReadError(t *testing.T) {
	h, _ := setup(t)
	fr := &failingReader{}
	body := io.MultiReader(strings.NewReader("name\nA\n"), fr)
	code, _ := post(t, h.ImportHabits, body)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
	if fr.calls > 10 {
		t.Fatalf("reader called %d times: import kept looping on a fatal error", fr.calls)
	}
}

// A2: NaN / Inf / negative values must not reach the database.
func TestImportEntriesRejectsInvalidValues(t *testing.T) {
	h, database := setup(t)
	insertHabit(t, database, "Sport", "numerical", 3, false)
	body := "habit_name,date,value\nSport,2025-01-01,Inf\nSport,2025-01-02,-1\nSport,2025-01-03,NaN\nSport,2025-01-04,2\n"
	code, out := post(t, h.ImportEntries, strings.NewReader(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out["imported"] != float64(1) {
		t.Errorf("imported = %v, want 1", out["imported"])
	}
	if n := errCount(t, out); n != 3 {
		t.Errorf("errors = %d, want 3", n)
	}
	var stored int
	if err := database.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 1 {
		t.Errorf("stored entries = %d, want 1", stored)
	}
}

func TestImportHabitsRejectsNaNAndInfGoal(t *testing.T) {
	h, _ := setup(t)
	body := "name,goal_value\nA,NaN\nB,Inf\nC,2\n"
	code, out := post(t, h.ImportHabits, strings.NewReader(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out["imported"] != float64(1) {
		t.Errorf("imported = %v, want 1", out["imported"])
	}
	if n := errCount(t, out); n != 2 {
		t.Errorf("errors = %d, want 2", n)
	}
}

func TestImportErrorListIsCapped(t *testing.T) {
	h, database := setup(t)
	insertHabit(t, database, "Sport", "numerical", 3, false)
	var b strings.Builder
	b.WriteString("habit_name,date,value\n")
	for i := 0; i < 150; i++ {
		b.WriteString("Inconnu,2025-01-01,1\n")
	}
	_, out := post(t, h.ImportEntries, strings.NewReader(b.String()))
	if n := errCount(t, out); n > maxImportErrors+1 {
		t.Errorf("errors = %d, want at most %d", n, maxImportErrors+1)
	}
}

// A8: an update must only change the columns present in the CSV header.
func TestImportHabitsPartialUpdateKeepsExistingFields(t *testing.T) {
	h, database := setup(t)
	id := insertHabit(t, database, "Sport", "numerical", 3, true)
	body := "id,name,group_name\n" + id + ",Sport,Sport-Groupe\n"
	code, _ := post(t, h.ImportHabits, strings.NewReader(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	var typ, unit, group string
	var goal float64
	var archived sql.NullString
	err := database.QueryRow(`SELECT type, goal_value, unit, group_name, archived_at FROM habits WHERE id=?`, id).
		Scan(&typ, &goal, &unit, &group, &archived)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "numerical" || goal != 3 || unit != "km" {
		t.Errorf("type/goal/unit overwritten: %s %v %s", typ, goal, unit)
	}
	if group != "Sport-Groupe" {
		t.Errorf("group_name = %q, want Sport-Groupe", group)
	}
	if !archived.Valid {
		t.Errorf("habit was unarchived by a CSV without archived_at column")
	}
}

// A8: an entry whose habit_id is unknown falls back to habit_name.
func TestImportEntriesFallsBackToNameWhenIDUnknown(t *testing.T) {
	h, database := setup(t)
	insertHabit(t, database, "Sport", "numerical", 3, false)
	body := "habit_id,habit_name,date,value\n" + uuid.NewString() + ",Sport,2025-01-01,2\n"
	code, out := post(t, h.ImportEntries, strings.NewReader(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if out["imported"] != float64(1) || errCount(t, out) != 0 {
		t.Errorf("imported=%v errors=%v, want 1 and none", out["imported"], out["errors"])
	}
}
