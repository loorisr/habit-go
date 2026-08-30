package entry

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	habitID := r.PathValue("id")
	date := r.PathValue("date")
	var req UpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date format YYYY-MM-DD", 400)
		return
	}
	if req.Value < 0 {
		http.Error(w, "value must be >= 0", 400)
		return
	}
	e, err := h.repo.Upsert(habitID, date, req.Value)
	if err != nil {
		// map FK error to 404
		if isForeignKeyError(err) {
			http.Error(w, "habit not found", 404)
			return
		}
		http.Error(w, "internal error", 500)
		return
	}
	jsonResponse(w, e)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	habitID := r.PathValue("id")
	date := r.PathValue("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date", 400)
		return
	}
	if err := h.repo.Delete(habitID, date); err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) GetRange(w http.ResponseWriter, r *http.Request) {
	habitID := r.PathValue("id")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			http.Error(w, "invalid from date", 400)
			return
		}
	}
	if to != "" {
		if _, err := time.Parse("2006-01-02", to); err != nil {
			http.Error(w, "invalid to date", 400)
			return
		}
	}
	entries, err := h.repo.GetRange(habitID, from, to)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	jsonResponse(w, entries)
}

func (h *Handler) Bulk(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			http.Error(w, "invalid from date", 400)
			return
		}
	}
	if to != "" {
		if _, err := time.Parse("2006-01-02", to); err != nil {
			http.Error(w, "invalid to date", 400)
			return
		}
	}
	habitID := r.URL.Query().Get("habit_id")
	var entries []Entry
	var err error
	if habitID != "" {
		entries, err = h.repo.GetRange(habitID, from, to)
	} else {
		// if no habit_id, need to fetch all? Use BulkGet with all habits? Simpler: query all entries in range
		entries, err = h.getAllInRange(from, to)
	}
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	jsonResponse(w, entries)
}

func (h *Handler) getAllInRange(from, to string) ([]Entry, error) {
	query := `SELECT id, habit_id, date, value, created_at, updated_at FROM entries WHERE 1=1`
	args := []interface{}{}
	if from != "" {
		query += ` AND date >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND date <= ?`
		args = append(args, to)
	}
	query += ` ORDER BY date ASC`
	rows, err := h.repo.db.Query(query, args...)
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

func isForeignKeyError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "foreign key") || strings.Contains(s, "constraint")
}

func jsonResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
