package csvhandler

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// dumpRows returns every row of the query as a string, for comparison.
func dumpRows(t *testing.T, database *sql.DB, query string) []string {
	t.Helper()
	rows, err := database.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprint(vals...))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func exportBody(t *testing.T, fn http.HandlerFunc) string {
	t.Helper()
	rec := httptest.NewRecorder()
	fn(rec, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d", rec.Code)
	}
	return rec.Body.String()
}

// Export then import into an empty database must give back the same habits and entries.
// The habits include the awkward cases: a comma and quotes in the name, an archived
// habit, a fractional goal, a negative habit with goal 0, and an empty group.
func TestExportImportRoundtrip(t *testing.T) {
	src, srcDB := setup(t)
	lecture := uuid.NewString()
	cigs := uuid.NewString()
	yoga := uuid.NewString()
	_, err := srcDB.Exec(`INSERT INTO habits (id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at) VALUES
		(?, 'Lecture', 'Loisirs', 'numerical', 2.5, 'weekly', 0, 'pages', NULL, '2025-01-01T08:00:00Z', '2025-01-01T08:00:00Z'),
		(?, 'Cigarettes', 'Santé', 'boolean', 0, 'daily', 1, '', '2025-02-01T10:00:00Z', '2025-01-02T08:00:00Z', '2025-01-02T08:00:00Z'),
		(?, 'Yoga, "doux"', '', 'boolean', 1, 'monthly', 0, '', NULL, '2025-01-03T08:00:00Z', '2025-01-03T08:00:00Z')`,
		lecture, cigs, yoga)
	if err != nil {
		t.Fatal(err)
	}
	_, err = srcDB.Exec(`INSERT INTO entries (id, habit_id, date, value, created_at, updated_at) VALUES
		(?, ?, '2025-01-01', 3.5, 'x', 'x'),
		(?, ?, '2025-01-02', 1, 'x', 'x'),
		(?, ?, '2025-01-03', 1, 'x', 'x')`,
		uuid.NewString(), lecture, uuid.NewString(), cigs, uuid.NewString(), yoga)
	if err != nil {
		t.Fatal(err)
	}

	habitsCSV := exportBody(t, src.ExportHabits)
	entriesCSV := exportBody(t, src.ExportEntries)

	dst, dstDB := setup(t)
	code, out := post(t, dst.ImportHabits, strings.NewReader(habitsCSV))
	if code != http.StatusOK || out["imported"] != float64(3) || errCount(t, out) != 0 {
		t.Fatalf("habits import: code=%d out=%v", code, out)
	}
	code, out = post(t, dst.ImportEntries, strings.NewReader(entriesCSV))
	if code != http.StatusOK || out["imported"] != float64(3) || errCount(t, out) != 0 {
		t.Fatalf("entries import: code=%d out=%v", code, out)
	}

	const habitsQ = `SELECT id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at FROM habits ORDER BY id`
	const entriesQ = `SELECT habit_id, date, value FROM entries ORDER BY habit_id, date`
	// guard against comparing two empty sets
	if n := len(dumpRows(t, srcDB, habitsQ)); n != 3 {
		t.Fatalf("source has %d habits, want 3", n)
	}
	if n := len(dumpRows(t, srcDB, entriesQ)); n != 3 {
		t.Fatalf("source has %d entries, want 3", n)
	}
	if got, want := dumpRows(t, dstDB, habitsQ), dumpRows(t, srcDB, habitsQ); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("habits after roundtrip:\n got  %v\n want %v", got, want)
	}
	if got, want := dumpRows(t, dstDB, entriesQ), dumpRows(t, srcDB, entriesQ); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("entries after roundtrip:\n got  %v\n want %v", got, want)
	}
}
