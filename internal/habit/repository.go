package habit

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

func (r *Repository) List(includeArchived bool) ([]Habit, error) {
	query := `SELECT id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at FROM habits`
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
		var h Habit
		var isNeg int
		var archived sql.NullString
		var unit sql.NullString
		if err := rows.Scan(&h.ID, &h.Name, &h.GroupName, &h.Type, &h.GoalValue, &h.GoalPeriod, &isNeg, &unit, &archived, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		h.IsNegative = isNeg != 0
		if unit.Valid {
			h.Unit = unit.String
		}
		if archived.Valid {
			h.ArchivedAt = &archived.String
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *Repository) Get(id string) (*Habit, error) {
	var h Habit
	var isNeg int
	var archived sql.NullString
	var unit sql.NullString
	err := r.db.QueryRow(`SELECT id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at FROM habits WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.GroupName, &h.Type, &h.GoalValue, &h.GoalPeriod, &isNeg, &unit, &archived, &h.CreatedAt, &h.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	h.IsNegative = isNeg != 0
	if unit.Valid {
		h.Unit = unit.String
	}
	if archived.Valid {
		h.ArchivedAt = &archived.String
	}
	return &h, nil
}

func (r *Repository) Create(req CreateRequest) (*Habit, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	isNeg := 0
	if req.IsNegative {
		isNeg = 1
	}
	unit := strings.TrimSpace(req.Unit)
	_, err := r.db.Exec(`INSERT INTO habits (id, name, group_name, type, goal_value, goal_period, is_negative, unit, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, strings.TrimSpace(req.Name), strings.TrimSpace(req.GroupName), req.Type, req.GoalValue, req.GoalPeriod, isNeg, unit, now, now)
	if err != nil {
		return nil, err
	}
	return r.Get(id)
}

func (r *Repository) Update(id string, req UpdateRequest) (*Habit, error) {
	h, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, ErrNotFound
	}
	name := h.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name required")
		}
		if len(name) > 100 {
			return nil, fmt.Errorf("name too long (max 100)")
		}
	}
	group := h.GroupName
	if req.GroupName != nil {
		group = strings.TrimSpace(*req.GroupName)
		if len(group) > 100 {
			return nil, fmt.Errorf("group_name too long (max 100)")
		}
	}
	typ := h.Type
	if req.Type != nil {
		if *req.Type != "boolean" && *req.Type != "numerical" {
			return nil, fmt.Errorf("invalid type")
		}
		typ = *req.Type
	}
	goalVal := h.GoalValue
	if req.GoalValue != nil {
		if *req.GoalValue <= 0 {
			return nil, fmt.Errorf("goal_value must be >0")
		}
		goalVal = *req.GoalValue
	}
	goalPeriod := h.GoalPeriod
	if req.GoalPeriod != nil {
		if *req.GoalPeriod != "daily" && *req.GoalPeriod != "weekly" && *req.GoalPeriod != "monthly" {
			return nil, fmt.Errorf("invalid goal_period")
		}
		goalPeriod = *req.GoalPeriod
	}
	isNeg := h.IsNegative
	if req.IsNegative != nil {
		isNeg = *req.IsNegative
	}
	unit := h.Unit
	if req.Unit != nil {
		unit = strings.TrimSpace(*req.Unit)
		if len(unit) > 20 {
			return nil, fmt.Errorf("unit too long (max 20)")
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	isNegInt := 0
	if isNeg {
		isNegInt = 1
	}
	_, err = r.db.Exec(`UPDATE habits SET name=?, group_name=?, type=?, goal_value=?, goal_period=?, is_negative=?, unit=?, updated_at=? WHERE id=?`,
		name, group, typ, goalVal, goalPeriod, isNegInt, unit, now, id)
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

func (r *Repository) FindByName(name string) (*Habit, error) {
	var h Habit
	var isNeg int
	var archived sql.NullString
	var unit sql.NullString
	err := r.db.QueryRow(`SELECT id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at FROM habits WHERE name=? LIMIT 1`, name).
		Scan(&h.ID, &h.Name, &h.GroupName, &h.Type, &h.GoalValue, &h.GoalPeriod, &isNeg, &unit, &archived, &h.CreatedAt, &h.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	h.IsNegative = isNeg != 0
	if unit.Valid {
		h.Unit = unit.String
	}
	if archived.Valid {
		h.ArchivedAt = &archived.String
	}
	return &h, nil
}

func validateCreate(req CreateRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("name required")
	}
	if len(name) > 100 {
		return fmt.Errorf("name too long (max 100)")
	}
	if len(req.Unit) > 20 {
		return fmt.Errorf("unit too long (max 20)")
	}
	if req.Type != "boolean" && req.Type != "numerical" {
		return fmt.Errorf("invalid type")
	}
	if req.GoalValue <= 0 || req.GoalValue > 1e9 {
		return fmt.Errorf("goal_value must be >0")
	}
	if req.GoalPeriod != "daily" && req.GoalPeriod != "weekly" && req.GoalPeriod != "monthly" {
		return fmt.Errorf("invalid goal_period")
	}
	return nil
}
