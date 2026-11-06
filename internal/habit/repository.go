package habit

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const selectColumns = `SELECT id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at FROM habits`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHabit(s rowScanner) (Habit, error) {
	var h Habit
	var isNeg int
	var archived sql.NullString
	err := s.Scan(&h.ID, &h.Name, &h.GroupName, &h.Type, &h.GoalValue, &h.GoalPeriod, &isNeg, &h.Unit, &archived, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return h, err
	}
	h.IsNegative = isNeg != 0
	if archived.Valid {
		h.ArchivedAt = &archived.String
	}
	return h, nil
}

func (r *Repository) List(includeArchived bool) ([]Habit, error) {
	query := selectColumns
	if !includeArchived {
		query += ` WHERE archived_at IS NULL`
	}
	query += ` ORDER BY group_name ASC, name ASC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Habit{}
	for rows.Next() {
		h, err := scanHabit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Get returns nil, nil when the habit does not exist.
func (r *Repository) Get(id string) (*Habit, error) {
	h, err := scanHabit(r.db.QueryRow(selectColumns+` WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *Repository) Create(req CreateRequest) (*Habit, error) {
	f := Fields{
		Name:       req.Name,
		GroupName:  req.GroupName,
		Type:       req.Type,
		GoalValue:  req.GoalValue,
		GoalPeriod: req.GoalPeriod,
		IsNegative: req.IsNegative,
		Unit:       req.Unit,
	}.Normalize()
	if err := f.Validate(); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`INSERT INTO habits (id, name, group_name, type, goal_value, goal_period, is_negative, unit, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, f.Name, f.GroupName, f.Type, f.GoalValue, f.GoalPeriod, boolInt(f.IsNegative), f.Unit, now, now)
	if err != nil {
		return nil, err
	}
	return r.Get(id)
}

// Update merges the request into the stored habit, then validates the result as a whole (C12).
func (r *Repository) Update(id string, req UpdateRequest) (*Habit, error) {
	h, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, ErrNotFound
	}
	f := Fields{
		Name:       h.Name,
		GroupName:  h.GroupName,
		Type:       h.Type,
		GoalValue:  h.GoalValue,
		GoalPeriod: h.GoalPeriod,
		IsNegative: h.IsNegative,
		Unit:       h.Unit,
	}
	if req.Name != nil {
		f.Name = *req.Name
	}
	if req.GroupName != nil {
		f.GroupName = *req.GroupName
	}
	if req.Type != nil {
		f.Type = *req.Type
	}
	if req.GoalValue != nil {
		f.GoalValue = *req.GoalValue
	}
	if req.GoalPeriod != nil {
		f.GoalPeriod = *req.GoalPeriod
	}
	if req.IsNegative != nil {
		f.IsNegative = *req.IsNegative
	}
	if req.Unit != nil {
		f.Unit = *req.Unit
	}
	f = f.Normalize()
	if err := f.Validate(); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.Exec(`UPDATE habits SET name=?, group_name=?, type=?, goal_value=?, goal_period=?, is_negative=?, unit=?, updated_at=? WHERE id=?`,
		f.Name, f.GroupName, f.Type, f.GoalValue, f.GoalPeriod, boolInt(f.IsNegative), f.Unit, now, id)
	if err != nil {
		return nil, err
	}
	return r.Get(id)
}

func (r *Repository) Archive(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`UPDATE habits SET archived_at=?, updated_at=? WHERE id=?`, now, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Restore(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`UPDATE habits SET archived_at=NULL, updated_at=? WHERE id=?`, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) HardDelete(id string) error {
	res, err := r.db.Exec(`DELETE FROM habits WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
