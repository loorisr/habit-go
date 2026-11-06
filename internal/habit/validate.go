package habit

import (
	"fmt"
	"math"
	"strings"
)

// ValidationError is an input error the client can fix (HTTP 400). C11.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// Fields are the user-editable attributes shared by the API and the CSV import. C12.
type Fields struct {
	Name       string
	GroupName  string
	Type       string
	GoalValue  float64
	GoalPeriod string
	IsNegative bool
	Unit       string
}

// Normalize trims the free-text fields.
func (f Fields) Normalize() Fields {
	f.Name = strings.TrimSpace(f.Name)
	f.GroupName = strings.TrimSpace(f.GroupName)
	f.Unit = strings.TrimSpace(f.Unit)
	return f
}

// Validate expects normalized fields. A7: goal 0 is allowed for negative habits only
// ("zero cigarettes"), so that a slip is a failure.
func (f Fields) Validate() error {
	if f.Name == "" {
		return invalid("name required")
	}
	if len(f.Name) > 100 {
		return invalid("name too long (max 100)")
	}
	if len(f.GroupName) > 100 {
		return invalid("group_name too long (max 100)")
	}
	if len(f.Unit) > 20 {
		return invalid("unit too long (max 20)")
	}
	if f.Type != "boolean" && f.Type != "numerical" {
		return invalid("invalid type")
	}
	if f.GoalPeriod != "daily" && f.GoalPeriod != "weekly" && f.GoalPeriod != "monthly" {
		return invalid("invalid goal_period")
	}
	g := f.GoalValue
	if math.IsNaN(g) || math.IsInf(g, 0) || g > 1e9 {
		return invalid("goal_value must be a finite number")
	}
	if f.IsNegative {
		if g < 0 {
			return invalid("goal_value must be >=0")
		}
	} else if g <= 0 {
		return invalid("goal_value must be >0")
	}
	return nil
}
