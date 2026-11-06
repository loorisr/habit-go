package habit

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"habit-go/internal/entry"
)

// maxRecentDays caps with_entries so a huge value cannot exhaust memory (B6).
const maxRecentDays = 366

// maxProgressDays is the longest goal window (monthly); one bulk query covers all periods.
const maxProgressDays = 30

type Handler struct {
	repo      *Repository
	entryRepo *entry.Repository
}

func NewHandler(repo *Repository, entryRepo *entry.Repository) *Handler {
	return &Handler{repo: repo, entryRepo: entryRepo}
}

// baseTime reads the optional ?today=YYYY-MM-DD param (client local date, avoids TZ divergence).
func baseTime(r *http.Request) (time.Time, bool) {
	if s := r.URL.Query().Get("today"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		return t, err == nil
	}
	return time.Now(), true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	baseNow, ok := baseTime(r)
	if !ok {
		http.Error(w, "invalid today param, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	withEntriesStr := r.URL.Query().Get("with_entries")
	n := 0
	if withEntriesStr != "" {
		v, err := strconv.Atoi(withEntriesStr)
		if err != nil || v <= 0 || v > maxRecentDays {
			http.Error(w, "invalid with_entries (1-366)", http.StatusBadRequest)
			return
		}
		n = v
	}

	habits, err := h.repo.List(includeArchived)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if habits == nil {
		habits = []Habit{}
	}
	progress, err := h.progressFor(habits, baseNow)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if n == 0 {
		out := make([]HabitWithEntries, 0, len(habits))
		for _, hab := range habits {
			out = append(out, HabitWithEntries{Habit: hab, Progress: progress[hab.ID]})
		}
		jsonResponse(w, out)
		return
	}

	from, to, dates := lastNDatesFrom(baseNow, n)
	ids := make([]string, len(habits))
	for i, hab := range habits {
		ids[i] = hab.ID
	}
	bulk, err := h.entryRepo.BulkGet(ids, from, to)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]HabitWithEntries, 0, len(habits))
	for _, hab := range habits {
		byDate := make(map[string]float64, len(bulk[hab.ID]))
		for _, e := range bulk[hab.ID] {
			byDate[e.Date] = e.Value
		}
		rec := []RecentEntry{}
		for _, d := range dates {
			if v, ok := byDate[d]; ok {
				rec = append(rec, RecentEntry{Date: d, Value: v})
			}
		}
		out = append(out, HabitWithEntries{Habit: hab, RecentEntries: rec, Progress: progress[hab.ID]})
	}
	jsonResponse(w, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	hab, err := h.repo.Create(req)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	jsonStatus(w, http.StatusCreated, hab)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	baseNow, ok := baseTime(r)
	if !ok {
		http.Error(w, "invalid today param", http.StatusBadRequest)
		return
	}
	hab, err := h.repo.Get(id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if hab == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	progress, err := h.progressFor([]Habit{*hab}, baseNow)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, HabitWithEntries{Habit: *hab, Progress: progress[hab.ID]})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	hab, err := h.repo.Update(id, req)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	jsonResponse(w, hab)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Archive(r.PathValue("id")); err != nil {
		writeRepoError(w, err)
		return
	}
	jsonResponse(w, map[string]string{"status": "archived"})
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Restore(r.PathValue("id")); err != nil {
		writeRepoError(w, err)
		return
	}
	jsonResponse(w, map[string]string{"status": "restored"})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("hard") != "true" {
		http.Error(w, "use POST /archive for soft delete; add ?hard=true for hard", http.StatusBadRequest)
		return
	}
	if err := h.repo.HardDelete(r.PathValue("id")); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "ok"})
}

// writeRepoError maps domain errors to HTTP status codes (C11: no more string matching).
func writeRepoError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		http.Error(w, ve.Msg, http.StatusBadRequest)
	case errors.Is(err, ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func jsonResponse(w http.ResponseWriter, v interface{}) {
	jsonStatus(w, http.StatusOK, v)
}

// jsonStatus sets Content-Type before WriteHeader so the header is actually sent (C13).
func jsonStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// progressFor computes the rolling progress of every habit with a single entries query (D6: no N+1).
func (h *Handler) progressFor(habits []Habit, base time.Time) (map[string]*Progress, error) {
	out := make(map[string]*Progress, len(habits))
	if len(habits) == 0 {
		return out, nil
	}
	from, to, _ := lastNDatesFrom(base, maxProgressDays)
	ids := make([]string, len(habits))
	for i, hab := range habits {
		ids[i] = hab.ID
	}
	bulk, err := h.entryRepo.BulkGet(ids, from, to)
	if err != nil {
		return nil, err
	}
	for i := range habits {
		out[habits[i].ID] = computeProgress(&habits[i], bulk[habits[i].ID], base)
	}
	return out, nil
}

func periodDays(period string) int {
	switch period {
	case "weekly":
		return 7
	case "monthly":
		return 30
	default:
		return 1
	}
}

// computeProgress sums the entries of the goal window ending at base.
// entries must be sorted or not: only the date window matters.
func computeProgress(hab *Habit, entries []entry.Entry, base time.Time) *Progress {
	from, to, _ := lastNDatesFrom(base, periodDays(hab.GoalPeriod))
	var current float64
	for _, e := range entries {
		if e.Date < from || e.Date > to {
			continue
		}
		if hab.Type == "boolean" {
			if e.Value >= 1 {
				current++
			}
		} else {
			current += e.Value
		}
	}
	var success bool
	if hab.IsNegative {
		success = current <= hab.GoalValue
	} else {
		success = current >= hab.GoalValue
	}
	// A7: goal 0 (negative habit) has no ratio; any entry above 0 means the goal is exceeded.
	percentage := 0.0
	if hab.GoalValue != 0 {
		percentage = current / hab.GoalValue * 100
	} else if current > 0 {
		percentage = 100
	}
	return &Progress{Current: current, Target: hab.GoalValue, Success: success, Period: hab.GoalPeriod, Percentage: percentage}
}

func lastNDatesFrom(base time.Time, n int) (from, to string, dates []string) {
	to = base.Format("2006-01-02")
	fromTime := base.AddDate(0, 0, -(n - 1))
	from = fromTime.Format("2006-01-02")
	for i := 0; i < n; i++ {
		dates = append(dates, fromTime.AddDate(0, 0, i).Format("2006-01-02"))
	}
	return
}
