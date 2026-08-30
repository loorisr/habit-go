package csvhandler

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"habit-go/internal/habit"
)

type Handler struct {
	db        *sql.DB
	habitRepo *habit.Repository
}

func NewHandler(db *sql.DB, hr *habit.Repository) *Handler {
	return &Handler{db: db, habitRepo: hr}
}

func (h *Handler) ExportHabits(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at FROM habits ORDER BY name`)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	// Buffer all records first to avoid partial CSV on scan errors
	records := [][]string{}
	for rows.Next() {
		var id, name, groupName, typ, goalPeriod, createdAt string
		var goalValue float64
		var isNeg int
		var unit sql.NullString
		var archived sql.NullString
		if err := rows.Scan(&id, &name, &groupName, &typ, &goalValue, &goalPeriod, &isNeg, &unit, &archived, &createdAt); err != nil {
			http.Error(w, "db scan error", 500)
			return
		}
		archStr := ""
		if archived.Valid {
			archStr = archived.String
		}
		unitStr := ""
		if unit.Valid {
			unitStr = unit.String
		}
		records = append(records, []string{id, name, groupName, typ, strconv.FormatFloat(goalValue, 'g', -1, 64), goalPeriod, strconv.Itoa(isNeg), unitStr, archStr, createdAt})
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=habits.csv")
	writer := csv.NewWriter(w)
	header := []string{"id", "name", "group_name", "type", "goal_value", "goal_period", "is_negative", "unit", "archived_at", "created_at"}
	if err := writer.Write(header); err != nil {
		http.Error(w, "csv write error", 500)
		return
	}
	for _, rec := range records {
		if err := writer.Write(rec); err != nil {
			http.Error(w, "csv write error", 500)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		http.Error(w, "csv flush error", 500)
		return
	}
}

func (h *Handler) ExportEntries(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT e.habit_id, h.name, e.date, e.value FROM entries e JOIN habits h ON h.id=e.habit_id ORDER BY h.name, e.date`)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	records := [][]string{}
	for rows.Next() {
		var habitID, habitName, date string
		var value float64
		if err := rows.Scan(&habitID, &habitName, &date, &value); err != nil {
			http.Error(w, "db scan error", 500)
			return
		}
		records = append(records, []string{habitID, habitName, date, strconv.FormatFloat(value, 'g', -1, 64)})
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=entries.csv")
	writer := csv.NewWriter(w)
	header := []string{"habit_id", "habit_name", "date", "value"}
	if err := writer.Write(header); err != nil {
		http.Error(w, "csv write error", 500)
		return
	}
	for _, rec := range records {
		if err := writer.Write(rec); err != nil {
			http.Error(w, "csv write error", 500)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		http.Error(w, "csv flush error", 500)
		return
	}
}

func (h *Handler) ImportHabits(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
		http.Error(w, "invalid multipart", 400)
		return
	}
	var reader *csv.Reader
	file, _, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		reader = csv.NewReader(file)
	} else {
		reader = csv.NewReader(r.Body)
	}
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		http.Error(w, "invalid csv header", 400)
		return
	}
	// strip BOM
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\xEF\xBB\xBF")
	}
	idx := headerIndex(header)
	// validate required columns
	if _, ok := idx["name"]; !ok {
		http.Error(w, "missing name column", 400)
		return
	}
	count := 0
	errorsArr := []string{}
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "db begin failed", 500)
		return
	}
	defer func() { _ = tx.Rollback() }()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorsArr = append(errorsArr, err.Error())
			continue
		}
		m := mapFromRecord(header, record, idx)
		id := strings.TrimSpace(m["id"])
		name := strings.TrimSpace(m["name"])
		if name == "" {
			errorsArr = append(errorsArr, "missing name")
			continue
		}
		if len(name) > 100 {
			errorsArr = append(errorsArr, "name too long: "+name)
			continue
		}
		groupName := strings.TrimSpace(m["group_name"])
		typ := strings.TrimSpace(m["type"])
		if typ == "" {
			typ = "boolean"
		}
		if typ != "boolean" && typ != "numerical" {
			errorsArr = append(errorsArr, "invalid type "+typ)
			continue
		}
		goalValStr := strings.TrimSpace(m["goal_value"])
		goalVal := 1.0
		if goalValStr != "" {
			if v, err := strconv.ParseFloat(goalValStr, 64); err == nil {
				if v <= 0 || v > 1e9 {
					errorsArr = append(errorsArr, "invalid goal_value "+goalValStr)
					continue
				}
				goalVal = v
			} else {
				errorsArr = append(errorsArr, "invalid goal_value "+goalValStr)
				continue
			}
		}
		goalPeriod := strings.TrimSpace(m["goal_period"])
		if goalPeriod == "" {
			goalPeriod = "daily"
		}
		if goalPeriod != "daily" && goalPeriod != "weekly" && goalPeriod != "monthly" {
			errorsArr = append(errorsArr, "invalid goal_period "+goalPeriod)
			continue
		}
		isNegStr := strings.TrimSpace(m["is_negative"])
		isNeg := 0
		if isNegStr == "1" || strings.EqualFold(isNegStr, "true") {
			isNeg = 1
		}
		unit := strings.TrimSpace(m["unit"])
		if len(unit) > 20 {
			errorsArr = append(errorsArr, "unit too long")
			continue
		}
		archived := strings.TrimSpace(m["archived_at"])
		var archVal interface{}
		if archived == "" {
			archVal = nil
		} else {
			if _, err := time.Parse(time.RFC3339, archived); err != nil {
				errorsArr = append(errorsArr, "invalid archived_at "+archived)
				continue
			}
			archVal = archived
		}
		createdAt := strings.TrimSpace(m["created_at"])
		if createdAt == "" {
			createdAt = time.Now().UTC().Format(time.RFC3339)
		} else {
			if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
				createdAt = time.Now().UTC().Format(time.RFC3339)
			}
		}
		updatedAt := time.Now().UTC().Format(time.RFC3339)

		var existingID string
		if id != "" {
			err := tx.QueryRow(`SELECT id FROM habits WHERE id=?`, id).Scan(&existingID)
			if err == nil {
				_, err = tx.Exec(`UPDATE habits SET name=?, group_name=?, type=?, goal_value=?, goal_period=?, is_negative=?, unit=?, archived_at=?, updated_at=? WHERE id=?`,
					name, groupName, typ, goalVal, goalPeriod, isNeg, unit, archVal, updatedAt, id)
				if err != nil {
					errorsArr = append(errorsArr, err.Error())
					continue
				}
				count++
				continue
			}
		}
		err = tx.QueryRow(`SELECT id FROM habits WHERE name=? LIMIT 1`, name).Scan(&existingID)
		if err == nil && existingID != "" {
			_, err = tx.Exec(`UPDATE habits SET name=?, group_name=?, type=?, goal_value=?, goal_period=?, is_negative=?, unit=?, archived_at=?, updated_at=? WHERE id=?`,
				name, groupName, typ, goalVal, goalPeriod, isNeg, unit, archVal, updatedAt, existingID)
			if err != nil {
				errorsArr = append(errorsArr, err.Error())
				continue
			}
			count++
			continue
		}
		newID := id
		if newID == "" {
			newID = uuid.NewString()
		} else {
			if _, err := uuid.Parse(newID); err != nil {
				newID = uuid.NewString()
			}
		}
		_, err = tx.Exec(`INSERT INTO habits (id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			newID, name, groupName, typ, goalVal, goalPeriod, isNeg, unit, archVal, createdAt, updatedAt)
		if err != nil {
			errorsArr = append(errorsArr, err.Error())
			continue
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "commit failed: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"imported": count, "errors": errorsArr})
}

func (h *Handler) ImportEntries(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
		http.Error(w, "invalid multipart", 400)
		return
	}
	var reader *csv.Reader
	file, _, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		reader = csv.NewReader(file)
	} else {
		reader = csv.NewReader(r.Body)
	}
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		http.Error(w, "invalid csv header", 400)
		return
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\xEF\xBB\xBF")
	}
	idx := headerIndex(header)
	count := 0
	errorsArr := []string{}
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "db begin failed", 500)
		return
	}
	defer func() { _ = tx.Rollback() }()
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorsArr = append(errorsArr, err.Error())
			continue
		}
		m := mapFromRecord(header, record, idx)
		habitID := strings.TrimSpace(m["habit_id"])
		habitName := strings.TrimSpace(m["habit_name"])
		date := strings.TrimSpace(m["date"])
		valStr := strings.TrimSpace(m["value"])
		if _, err := time.Parse("2006-01-02", date); err != nil {
			errorsArr = append(errorsArr, "invalid date "+date)
			continue
		}
		val := 0.0
		if valStr != "" {
			if v, err := strconv.ParseFloat(valStr, 64); err == nil {
				val = v
			} else {
				errorsArr = append(errorsArr, "invalid value "+valStr)
				continue
			}
		}
		resolvedID := habitID
		if resolvedID == "" && habitName != "" {
			err := tx.QueryRow(`SELECT id FROM habits WHERE name=? LIMIT 1`, habitName).Scan(&resolvedID)
			if err != nil {
				errorsArr = append(errorsArr, "habit not found: "+habitName)
				continue
			}
		}
		if resolvedID == "" {
			errorsArr = append(errorsArr, "missing habit_id and habit_name")
			continue
		}
		if _, err := uuid.Parse(resolvedID); err != nil {
			errorsArr = append(errorsArr, "invalid habit_id "+resolvedID)
			continue
		}
		var exists string
		if err := tx.QueryRow(`SELECT id FROM habits WHERE id=?`, resolvedID).Scan(&exists); err != nil {
			errorsArr = append(errorsArr, "habit_id not found: "+resolvedID)
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		newID := uuid.NewString()
		_, err = tx.Exec(`INSERT INTO entries (id, habit_id, date, value, created_at, updated_at) VALUES (?,?,?,?,?,?)
			ON CONFLICT(habit_id, date) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
			newID, resolvedID, date, val, now, now)
		if err != nil {
			errorsArr = append(errorsArr, err.Error())
			continue
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "commit failed: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"imported": count, "errors": errorsArr})
}

func headerIndex(header []string) map[string]int {
	m := make(map[string]int)
	for i, h := range header {
		m[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return m
}

func mapFromRecord(header []string, record []string, idx map[string]int) map[string]string {
	out := make(map[string]string)
	for _, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		i := idx[key]
		if i < len(record) {
			out[key] = record[i]
		}
	}
	return out
}
