package timetable

import "context"

// ClassArrivalQuery supplies the recurring dismissal times used by Care Plan.
// Class keys are trimmed and lowercased; weekday keys and HH:MM values retain
// their persisted form. Only requested classes in the current tenant return.
type ClassArrivalQuery interface {
	ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, error)
}
