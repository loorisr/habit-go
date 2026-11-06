package csvhandler

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"habit-go/internal/habit"

	"github.com/google/uuid"
)

// maxImportErrors caps the error list returned to the client (A1).
const maxImportErrors = 100

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
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

// importInput is the CSV source of an import request: multipart "file" or raw body.
type importInput struct {
	reader *csv.Reader
	header []string
	idx    map[string]int
	close  func()
}

func newImportInput(w http.ResponseWriter, r *http.Request) (*importInput, bool) {
	if err := r.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
		http.Error(w, "invalid multipart", http.StatusBadRequest)
		return nil, false
	}
	var src io.Reader = r.Body
	closeFn := func() {}
	if f, _, err := r.FormFile("file"); err == nil {
		src = f
		closeFn = func() { f.Close() }
	}
	reader := csv.NewReader(src)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		closeFn()
		http.Error(w, "invalid csv header", http.StatusBadRequest)
		return nil, false
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\xEF\xBB\xBF")
	}
	return &importInput{reader: reader, header: header, idx: headerIndex(header), close: closeFn}, true
}

// nextRecord returns the next record with its line number. Malformed lines are
// recorded in errs and skipped. A failing body (client gone, size limit) is
// returned as fatal: the reader would keep returning the same error forever (A1).
func (in *importInput) nextRecord(errs *errList) (rec []string, line int, done bool, fatal error) {
	for {
		record, err := in.reader.Read()
		if err == io.EOF {
			return nil, 0, true, nil
		}
		if err == nil {
			l, _ := in.reader.FieldPos(0)
			return record, l, false, nil
		}
		var pe *csv.ParseError
		if errors.As(err, &pe) {
			errs.add("line %d: %v", pe.Line, pe.Err)
			continue
		}
		return nil, 0, false, err
	}
}

func writeReadError(w http.ResponseWriter, err error) {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "read error", http.StatusBadRequest)
}

// errList keeps at most maxImportErrors messages.
type errList struct {
	msgs    []string
	dropped int
}

func (e *errList) add(format string, args ...any) {
	if len(e.msgs) >= maxImportErrors {
		e.dropped++
		return
	}
	e.msgs = append(e.msgs, fmt.Sprintf(format, args...))
}

func (e *errList) result() []string {
	out := make([]string, len(e.msgs), len(e.msgs)+1)
	copy(out, e.msgs)
	if e.dropped > 0 {
		out = append(out, fmt.Sprintf("... %d more errors not shown", e.dropped))
	}
	return out
}

func (h *Handler) ImportHabits(w http.ResponseWriter, r *http.Request) {
	in, ok := newImportInput(w, r)
	if !ok {
		return
	}
	defer in.close()
	if _, ok := in.idx["name"]; !ok {
		http.Error(w, "missing name column", http.StatusBadRequest)
		return
	}
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "db begin failed", 500)
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0
	var errs errList
	for {
		rec, line, done, fatal := in.nextRecord(&errs)
		if fatal != nil {
			writeReadError(w, fatal)
			return
		}
		if done {
			break
		}
		m := mapFromRecord(in.header, rec, in.idx)
		id := strings.TrimSpace(m["id"])
		name := strings.TrimSpace(m["name"])

		// Target row: same id, else same name (historical upsert rule), else insert.
		targetID, err := lookupID(tx, `SELECT id FROM habits WHERE id=?`, id)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		if targetID == "" && name != "" {
			if targetID, err = lookupID(tx, `SELECT id FROM habits WHERE name=? LIMIT 1`, name); err != nil {
				http.Error(w, "db error", 500)
				return
			}
		}

		// Start from the stored row on update, from defaults on insert.
		f := habit.Fields{Type: "boolean", GoalValue: 1, GoalPeriod: "daily"}
		var archived sql.NullString
		createdAt := now
		if targetID != "" {
			f, archived, createdAt, err = loadHabit(tx, targetID)
			if err != nil {
				http.Error(w, "db error", 500)
				return
			}
		}

		// A8: only columns present in the header (and in this record) override the current value.
		if v, ok := m["name"]; ok {
			f.Name = v
		}
		if v, ok := m["group_name"]; ok {
			f.GroupName = v
		}
		if v, ok := m["type"]; ok {
			f.Type = orDefault(strings.TrimSpace(v), "boolean")
		}
		if v, ok := m["goal_value"]; ok {
			s := strings.TrimSpace(v)
			if s == "" {
				f.GoalValue = 1
			} else {
				g, err := strconv.ParseFloat(s, 64)
				if err != nil {
					errs.add("line %d: invalid goal_value %s", line, s)
					continue
				}
				f.GoalValue = g
			}
		}
		if v, ok := m["goal_period"]; ok {
			f.GoalPeriod = orDefault(strings.TrimSpace(v), "daily")
		}
		if v, ok := m["is_negative"]; ok {
			s := strings.TrimSpace(v)
			f.IsNegative = s == "1" || strings.EqualFold(s, "true")
		}
		if v, ok := m["unit"]; ok {
			f.Unit = v
		}
		if v, ok := m["archived_at"]; ok {
			s := strings.TrimSpace(v)
			archived = sql.NullString{}
			if s != "" {
				if _, err := time.Parse(time.RFC3339, s); err != nil {
					errs.add("line %d: invalid archived_at %s", line, s)
					continue
				}
				archived = sql.NullString{String: s, Valid: true}
			}
		}
		if targetID == "" {
			if v := strings.TrimSpace(m["created_at"]); v != "" {
				if _, err := time.Parse(time.RFC3339, v); err == nil {
					createdAt = v
				}
			}
		}

		f = f.Normalize()
		if err := f.Validate(); err != nil {
			errs.add("line %d: %v", line, err)
			continue
		}

		if targetID != "" {
			_, err = tx.Exec(`UPDATE habits SET name=?, group_name=?, type=?, goal_value=?, goal_period=?, is_negative=?, unit=?, archived_at=?, updated_at=? WHERE id=?`,
				f.Name, f.GroupName, f.Type, f.GoalValue, f.GoalPeriod, boolInt(f.IsNegative), f.Unit, archived, now, targetID)
		} else {
			newID := id
			if _, perr := uuid.Parse(newID); perr != nil {
				newID = uuid.NewString()
			}
			_, err = tx.Exec(`INSERT INTO habits (id, name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
				newID, f.Name, f.GroupName, f.Type, f.GoalValue, f.GoalPeriod, boolInt(f.IsNegative), f.Unit, archived, createdAt, now)
		}
		if err != nil {
			errs.add("line %d: %v", line, err)
			continue
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "commit failed: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"imported": count, "errors": errs.result()})
}

func (h *Handler) ImportEntries(w http.ResponseWriter, r *http.Request) {
	in, ok := newImportInput(w, r)
	if !ok {
		return
	}
	defer in.close()
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "db begin failed", 500)
		return
	}
	defer func() { _ = tx.Rollback() }()

	count := 0
	var errs errList
	for {
		rec, line, done, fatal := in.nextRecord(&errs)
		if fatal != nil {
			writeReadError(w, fatal)
			return
		}
		if done {
			break
		}
		m := mapFromRecord(in.header, rec, in.idx)
		date := strings.TrimSpace(m["date"])
		if _, err := time.Parse("2006-01-02", date); err != nil {
			errs.add("line %d: invalid date %s", line, date)
			continue
		}
		val := 0.0
		if s := strings.TrimSpace(m["value"]); s != "" {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil || !validEntryValue(v) {
				errs.add("line %d: invalid value %s", line, s)
				continue
			}
			val = v
		}
		habitID, userErr, dbErr := resolveHabit(tx, strings.TrimSpace(m["habit_id"]), strings.TrimSpace(m["habit_name"]))
		if dbErr != nil {
			http.Error(w, "db error", 500)
			return
		}
		if userErr != "" {
			errs.add("line %d: %s", line, userErr)
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`INSERT INTO entries (id, habit_id, date, value, created_at, updated_at) VALUES (?,?,?,?,?,?)
			ON CONFLICT(habit_id, date) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
			uuid.NewString(), habitID, date, val, now, now)
		if err != nil {
			errs.add("line %d: %v", line, err)
			continue
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "commit failed: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"imported": count, "errors": errs.result()})
}

// resolveHabit finds the habit by id, then falls back to name (A8).
// It returns a user-facing message for input errors and a separate error for database failures.
func resolveHabit(tx *sql.Tx, habitID, habitName string) (id, userErr string, dbErr error) {
	if habitID != "" {
		found, err := lookupID(tx, `SELECT id FROM habits WHERE id=?`, habitID)
		if err != nil {
			return "", "", err
		}
		if found != "" {
			return found, "", nil
		}
		if habitName == "" {
			return "", "habit_id not found: " + habitID, nil
		}
	}
	if habitName == "" {
		return "", "missing habit_id and habit_name", nil
	}
	found, err := lookupID(tx, `SELECT id FROM habits WHERE name=? LIMIT 1`, habitName)
	if err != nil {
		return "", "", err
	}
	if found == "" {
		return "", "habit not found: " + habitName, nil
	}
	return found, "", nil
}

// lookupID returns "" when no row matches.
func lookupID(tx *sql.Tx, query, arg string) (string, error) {
	var id string
	err := tx.QueryRow(query, arg).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func loadHabit(tx *sql.Tx, id string) (habit.Fields, sql.NullString, string, error) {
	var f habit.Fields
	var isNeg int
	var archived sql.NullString
	var created string
	err := tx.QueryRow(`SELECT name, group_name, type, goal_value, goal_period, is_negative, unit, archived_at, created_at FROM habits WHERE id=?`, id).
		Scan(&f.Name, &f.GroupName, &f.Type, &f.GoalValue, &f.GoalPeriod, &isNeg, &f.Unit, &archived, &created)
	f.IsNegative = isNeg != 0
	return f, archived, created, err
}

// validEntryValue mirrors the API rule (value >= 0) and rejects NaN and Inf.
func validEntryValue(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
