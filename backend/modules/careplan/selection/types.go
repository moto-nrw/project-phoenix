package selection

import (
	"fmt"
	"slices"
)

type Child struct {
	TargetGradeLevel         *int16
	OfferingIDs              []int64
	OfferingDays             []DaySelection
	ExcludedAutoAddTargetIDs map[int64]bool
}
type DaySelection struct {
	OfferingID   int64
	SelectedDays []string
}
type Selection struct {
	OfferingID            int64
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
}
type Grandfathered struct {
	Manual    map[int64]bool
	Automatic map[int64]bool
}
type Offering struct {
	ID                        int64
	SortOrder                 int
	DaysOfWeekMode            string
	AvailableDays             []string
	CountsAsCare              bool
	IncludesLunch             bool
	IsRequired                bool
	SelectionGroup            string
	SelectionRule             string
	AutoAddTriggerOfferingIDs []int64
	AutoAddGradeLevels        []int
	AvailabilityRule          *AvailabilityRule
}
type AvailabilityRule struct {
	Match      string                  `json:"match"`
	Conditions []AvailabilityCondition `json:"conditions"`
}
type AvailabilityCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    []int  `json:"value"`
}

func (r *AvailabilityRule) NormalizeAndValidate() error {
	if r == nil || len(r.Conditions) == 0 {
		return nil
	}
	if r.Match != "all" && r.Match != "any" {
		return fmt.Errorf("availability_rule.match must be %q or %q", "all", "any")
	}
	for i := range r.Conditions {
		condition := &r.Conditions[i]
		if condition.Source != "grade_level" {
			return fmt.Errorf("availability_rule condition %d has unknown source %q", i+1, condition.Source)
		}
		if condition.Operator != "in" && condition.Operator != "not_in" {
			return fmt.Errorf("availability_rule condition %d has unknown operator %q", i+1, condition.Operator)
		}
		if len(condition.Value) == 0 {
			return fmt.Errorf("availability_rule condition %d requires at least one value", i+1)
		}
		seen := make(map[int]struct{}, len(condition.Value))
		values := make([]int, 0, len(condition.Value))
		for _, grade := range condition.Value {
			if grade < 1 || grade > 13 {
				return fmt.Errorf("availability_rule condition %d contains invalid grade %d", i+1, grade)
			}
			if _, ok := seen[grade]; ok {
				continue
			}
			seen[grade] = struct{}{}
			values = append(values, grade)
		}
		slices.Sort(values)
		condition.Value = values
	}
	return nil
}

func (r *AvailabilityRule) MatchesGradeLevel(gradeLevel *int16) (bool, error) {
	if r == nil || len(r.Conditions) == 0 {
		return true, nil
	}
	if err := r.NormalizeAndValidate(); err != nil {
		return false, err
	}
	if gradeLevel == nil {
		return false, nil
	}
	matches := func(condition AvailabilityCondition) bool {
		included := slices.Contains(condition.Value, int(*gradeLevel))
		if condition.Operator == "not_in" {
			return !included
		}
		return included
	}
	if r.Match == "all" {
		for _, condition := range r.Conditions {
			if !matches(condition) {
				return false, nil
			}
		}
		return true, nil
	}
	for _, condition := range r.Conditions {
		if matches(condition) {
			return true, nil
		}
	}
	return false, nil
}
