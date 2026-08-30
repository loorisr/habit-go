package entry

type Entry struct {
	ID        string  `json:"id"`
	HabitID   string  `json:"habit_id"`
	Date      string  `json:"date"`
	Value     float64 `json:"value"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type UpsertRequest struct {
	Value float64 `json:"value"`
}
