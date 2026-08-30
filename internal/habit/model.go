package habit

type Habit struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	GroupName  string  `json:"group_name"`
	Type       string  `json:"type"`
	GoalValue  float64 `json:"goal_value"`
	GoalPeriod string  `json:"goal_period"`
	IsNegative bool    `json:"is_negative"`
	Unit       string  `json:"unit"`
	ArchivedAt *string `json:"archived_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type HabitWithEntries struct {
	Habit
	RecentEntries []RecentEntry `json:"recent_entries,omitempty"`
	Progress      *Progress     `json:"progress,omitempty"`
}

type RecentEntry struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type Progress struct {
	Current    float64 `json:"current"`
	Target     float64 `json:"target"`
	Success    bool    `json:"success"`
	Period     string  `json:"period"`
	Percentage float64 `json:"percentage"`
}

type CreateRequest struct {
	Name       string  `json:"name"`
	GroupName  string  `json:"group_name"`
	Type       string  `json:"type"`
	GoalValue  float64 `json:"goal_value"`
	GoalPeriod string  `json:"goal_period"`
	IsNegative bool    `json:"is_negative"`
	Unit       string  `json:"unit"`
}

type UpdateRequest struct {
	Name       *string  `json:"name"`
	GroupName  *string  `json:"group_name"`
	Type       *string  `json:"type"`
	GoalValue  *float64 `json:"goal_value"`
	GoalPeriod *string  `json:"goal_period"`
	IsNegative *bool    `json:"is_negative"`
	Unit       *string  `json:"unit"`
}
