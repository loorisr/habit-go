package entry

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(habitID, date string, value float64) (*Entry, error) {
	if len(date) != 10 {
		return nil, fmt.Errorf("invalid date: expected YYYY-MM-DD")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.NewString()
	// Try insert, on conflict update
	_, err := r.db.Exec(`INSERT INTO entries (id, habit_id, date, value, created_at, updated_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT(habit_id, date) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		id, habitID, date, value, now, now)
	if err != nil {
		return nil, err
	}
	return r.Get(habitID, date)
}

func (r *Repository) Get(habitID, date string) (*Entry, error) {
	var e Entry
	err := r.db.QueryRow(`SELECT id, habit_id, date, value, created_at, updated_at FROM entries WHERE habit_id=? AND date=?`, habitID, date).
		Scan(&e.ID, &e.HabitID, &e.Date, &e.Value, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) Delete(habitID, date string) error {
	_, err := r.db.Exec(`DELETE FROM entries WHERE habit_id=? AND date=?`, habitID, date)
	return err
}

func (r *Repository) GetRange(habitID, from, to string) ([]Entry, error) {
	query := `SELECT id, habit_id, date, value, created_at, updated_at FROM entries WHERE habit_id=?`
	args := []interface{}{habitID}
	if from != "" {
		query += ` AND date >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND date <= ?`
		args = append(args, to)
	}
	query += ` ORDER BY date ASC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.HabitID, &e.Date, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) BulkGet(habitIDs []string, from, to string) (map[string][]Entry, error) {
	if len(habitIDs) == 0 {
		return map[string][]Entry{}, nil
	}
	// Build IN clause
	// Use placeholders
	query := `SELECT id, habit_id, date, value, created_at, updated_at FROM entries WHERE habit_id IN (`
	args := []interface{}{}
	for i, id := range habitIDs {
		if i > 0 {
			query += `,`
		}
		query += `?`
		args = append(args, id)
	}
	query += `)`
	if from != "" {
		query += ` AND date >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND date <= ?`
		args = append(args, to)
	}
	query += ` ORDER BY habit_id, date ASC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string][]Entry)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.HabitID, &e.Date, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		m[e.HabitID] = append(m[e.HabitID], e)
	}
	return m, rows.Err()
}

func (r *Repository) GetByHabitDate(habitID string) ([]Entry, error) {
	return r.GetRange(habitID, "", "")
}
