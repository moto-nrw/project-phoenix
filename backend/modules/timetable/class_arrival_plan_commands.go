package timetable

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidClassArrivalPlan = errors.New("invalid class arrival plan")

type ClassArrivalPlanInput struct {
	SchoolClass  string
	ArrivalTimes map[string]string
	UpdatedBy    *int64
}

func (v ClassArrivalPlanInput) Valid() bool {
	if strings.TrimSpace(v.SchoolClass) == "" || v.UpdatedBy != nil && *v.UpdatedBy <= 0 {
		return false
	}
	for day, clock := range v.ArrivalTimes {
		switch strings.ToLower(strings.TrimSpace(day)) {
		case "mon", "tue", "wed", "thu", "fri":
		default:
			return false
		}
		if _, err := time.Parse("15:04", clock); err != nil {
			return false
		}
	}
	return true
}
