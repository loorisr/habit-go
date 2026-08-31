package habit

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"habit-go/internal/entry"
)

type Handler struct {
	repo      *Repository
	entryRepo *entry.Repository
}

func NewHandler(repo *Repository, entryRepo *entry.Repository) *Handler {
	return &Handler{repo: repo, entryRepo: entryRepo}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	withEntriesStr := r.URL.Query().Get("with_entries")
	// today param allows client to send its local date to avoid TZ divergence (YYYY-MM-DD)
	todayParam := r.URL.Query().Get("today")
	baseNow := time.Now()
	if todayParam != "" {
		if t, err := time.Parse("2006-01-02", todayParam); err == nil {
			baseNow = t
		} else {
			http.Error(w, "invalid today param, expected YYYY-MM-DD", 400)
			return
		}
	}
	habits, err := h.repo.List(includeArchived)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	if withEntriesStr != "" {
		n, err := strconv.Atoi(withEntriesStr)
		if err != nil || n <= 0 {
			http.Error(w, "invalid with_entries", 400)
			return
		}
		from, to, dates := lastNDatesFrom(baseNow, n)
		ids := make([]string, len(habits))
		for i, hab := range habits {
			ids[i] = hab.ID
		}
		bulk, err := h.entryRepo.BulkGet(ids, from, to)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		// Map entries by habit+date for quick lookup, fill recent_entries in order of dates + compute progress
		out := []HabitWithEntries{}
		for _, hab := range habits {
			entriesMap := make(map[string]float64)
			for _, e := range bulk[hab.ID] {
				entriesMap[e.Date] = e.Value
			}
			rec := []RecentEntry{}
			for _, d := range dates {
				if v, ok := entriesMap[d]; ok {
					rec = append(rec, RecentEntry{Date: d, Value: v})
				}
			}
			progress, err := h.computeProgressWithBase(&hab, baseNow)
			if err != nil {
				// log but still return habit without progress
				progress = nil
			}
			out = append(out, HabitWithEntries{Habit: hab, RecentEntries: rec, Progress: progress})
		}
		jsonResponse(w, out)
		return
	}
	// without with_entries, still include progress for main page needs
	if habits == nil {
		habits = []Habit{}
	}
	// include progress for each habit (needed for weekly/monthly red coloring on main page)
	out := []HabitWithEntries{}
	for _, hab := range habits {
		progress, err := h.computeProgressWithBase(&hab, baseNow)
		if err != nil {
			progress = nil
		}
		h := hab
		out = append(out, HabitWithEntries{Habit: h, Progress: progress})
	}
	jsonResponse(w, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	hab, err := h.repo.Create(req)
	if err != nil {
		// validation errors are 400, internal is 500
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "too long") {
			http.Error(w, err.Error(), 400)
		} else {
			http.Error(w, "internal error", 500)
		}
		return
	}
	w.WriteHeader(201)
	jsonResponse(w, hab)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	todayParam := r.URL.Query().Get("today")
	baseNow := time.Now()
	if todayParam != "" {
		if t, err := time.Parse("2006-01-02", todayParam); err == nil {
			baseNow = t
		} else {
			http.Error(w, "invalid today param", 400)
			return
		}
	}
	hab, err := h.repo.Get(id)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	if hab == nil {
		http.Error(w, "not found", 404)
		return
	}
	// Compute progress based on goal_period rolling window
	progress, err := h.computeProgressWithBase(hab, baseNow)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	resp := HabitWithEntries{
		Habit:    *hab,
		Progress: progress,
	}
	jsonResponse(w, resp)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	hab, err := h.repo.Update(id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", 404)
			return
		}
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "too long") {
			http.Error(w, err.Error(), 400)
			return
		}
		http.Error(w, "internal error", 500)
		return
	}
	jsonResponse(w, hab)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.Archive(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", 404)
			return
		}
		http.Error(w, "internal error", 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "archived"})
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.Restore(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", 404)
			return
		}
		http.Error(w, "internal error", 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "restored"})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	hard := r.URL.Query().Get("hard") == "true"
	if !hard {
		http.Error(w, "use POST /archive for soft delete; add ?hard=true for hard", 400)
		return
	}
	if err := h.repo.HardDelete(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", 404)
			return
		}
		http.Error(w, "internal error", 500)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "ok"})
}

func jsonResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// progress computation uses entry repo rolling window
func (h *Handler) computeProgress(hab *Habit) (*Progress, error) {
	return h.computeProgressWithBase(hab, time.Now())
}

func (h *Handler) computeProgressWithBase(hab *Habit, base time.Time) (*Progress, error) {
	var days int
	switch hab.GoalPeriod {
	case "daily":
		days = 1
	case "weekly":
		days = 7
	case "monthly":
		days = 30
	default:
		days = 1
	}
	from, to, _ := lastNDatesFrom(base, days)
	entries, err := h.entryRepo.GetRange(hab.ID, from, to)
	if err != nil {
		return nil, err
	}
	var current float64
	if hab.Type == "boolean" {
		// count yes (value !=0) -> each entry with value >=1 counts
		for _, e := range entries {
			if e.Value >= 1 {
				current += 1
			}
		}
	} else {
		for _, e := range entries {
			current += e.Value
		}
	}
	success := false
	if hab.IsNegative {
		success = current <= hab.GoalValue
	} else {
		success = current >= hab.GoalValue
	}
	percentage := 0.0
	if hab.GoalValue != 0 {
		percentage = current / hab.GoalValue * 100
	}
	return &Progress{Current: current, Target: hab.GoalValue, Success: success, Period: hab.GoalPeriod, Percentage: percentage}, nil
}

func lastNDates(n int) (from, to string, dates []string) {
	return lastNDatesFrom(time.Now(), n)
}

func lastNDatesFrom(base time.Time, n int) (from, to string, dates []string) {
	to = base.Format("2006-01-02")
	fromTime := base.AddDate(0, 0, -(n - 1))
	from = fromTime.Format("2006-01-02")
	for i := 0; i < n; i++ {
		d := fromTime.AddDate(0, 0, i).Format("2006-01-02")
		dates = append(dates, d)
	}
	return
}
