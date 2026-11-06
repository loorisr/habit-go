package db

import (
	"path/filepath"
	"testing"
)

// C14: foreign keys must hold on every connection, not only the first one.
func TestForeignKeysOnEveryConnection(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	// no idle connections: each query below may open a fresh connection
	database.SetMaxIdleConns(0)

	for i := 0; i < 3; i++ {
		var fk int
		if err := database.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if fk != 1 {
			t.Fatalf("attempt %d: foreign_keys = %d, want 1", i, fk)
		}
	}

	_, err = database.Exec(`INSERT INTO habits (id, name, type, goal_value, goal_period, updated_at) VALUES ('h1','h','boolean',1,'daily','2025-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO entries (id, habit_id, date, value, created_at, updated_at) VALUES ('e1','h1','2025-01-01',1,'x','x')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`DELETE FROM habits WHERE id='h1'`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("entries after cascade = %d, want 0", n)
	}
}

// A7: the one-time migration sets goal 0 on daily negative boolean habits, and runs only once.
func TestMigrateNegativeZeroGoal(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	insert := func(id, typ, period string, neg int, goal float64) {
		t.Helper()
		_, err := database.Exec(`INSERT INTO habits (id, name, type, goal_value, goal_period, is_negative, updated_at) VALUES (?,?,?,?,?,?,'2025-01-01T00:00:00Z')`,
			id, id, typ, goal, period, neg)
		if err != nil {
			t.Fatal(err)
		}
	}
	insert("cig", "boolean", "daily", 1, 1)
	insert("sport", "numerical", "daily", 1, 3) // numerical: untouched
	insert("pos", "boolean", "daily", 0, 1)     // positive: untouched

	// Simulate a database created before the migration.
	if _, err := database.Exec(`PRAGMA user_version=0`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	goal := func(id string) float64 {
		t.Helper()
		var g float64
		if err := database.QueryRow(`SELECT goal_value FROM habits WHERE id=?`, id).Scan(&g); err != nil {
			t.Fatal(err)
		}
		return g
	}
	if goal("cig") != 0 {
		t.Errorf("cig goal = %v, want 0", goal("cig"))
	}
	if goal("sport") != 3 || goal("pos") != 1 {
		t.Errorf("other habits changed: sport=%v pos=%v", goal("sport"), goal("pos"))
	}

	// Second run must not touch data again.
	if _, err := database.Exec(`UPDATE habits SET goal_value=1 WHERE id='cig'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if goal("cig") != 1 {
		t.Errorf("migration re-ran: cig goal = %v, want 1", goal("cig"))
	}
}
