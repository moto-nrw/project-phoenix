package schedule

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// int64FilterArgs widens IDs for the persistence-neutral Filter.In API.
func int64FilterArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

func dateFilterArgs(dates []timezone.Date) []any {
	args := make([]any, len(dates))
	for i, date := range dates {
		args[i] = date
	}
	return args
}
