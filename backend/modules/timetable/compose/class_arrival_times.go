package compose

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// NewClassArrivalQueries binds the read without constructing activity mutation
// collaborators. Timetable retains ownership of class dismissal persistence.
func NewClassArrivalQueries(db *bun.DB, observe func(Observation)) (timetable.ClassArrivalQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("class arrival queries: database and observer are required")
	}
	return classArrivalQueries{store: postgres.New(databaseRuntime(db)), observe: observe}, nil
}

type classArrivalQueries struct {
	store   *postgres.Store
	observe func(Observation)
}

func (q classArrivalQueries) ListClassArrivalTimes(ctx context.Context, classes []string) (map[string]map[string]string, error) {
	rows, err := q.ListClassArrivalPlans(ctx, classes)
	if err != nil {
		return nil, err
	}
	times := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		times[strings.ToLower(strings.TrimSpace(row.SchoolClass))] = row.ArrivalTimes
	}
	return times, nil
}

func (q classArrivalQueries) ListClassArrivalPlans(ctx context.Context, classes []string) ([]timetable.ClassArrivalPlan, error) {
	keys := classArrivalKeys(classes)
	if len(keys) == 0 {
		return []timetable.ClassArrivalPlan{}, nil
	}
	started := time.Now()
	rows, stats, err := q.store.ListClassArrivalPlans(ctx, keys)
	q.observe(Observation{Operation: "list_class_arrival_times", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return nil, err
	}
	result := make([]timetable.ClassArrivalPlan, len(rows))
	for i, row := range rows {
		result[i] = nativeClassArrivalPlan(row)
	}
	return result, nil
}

func (q classArrivalQueries) ListClassArrivalExceptions(ctx context.Context, classes []string, from, to string) ([]timetable.ClassArrivalException, error) {
	if !timetable.ValidClassArrivalExceptionDates(from, to) {
		return nil, timetable.ErrInvalidClassArrivalExceptionRange
	}
	keys := classArrivalKeys(classes)
	if len(keys) == 0 || to < from {
		return []timetable.ClassArrivalException{}, nil
	}
	started := time.Now()
	rows, stats, err := q.store.ListClassArrivalExceptionRecords(ctx, keys, from, to)
	q.observe(Observation{Operation: "list_class_arrival_exceptions", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return nil, err
	}
	result := make([]timetable.ClassArrivalException, len(rows))
	for i, row := range rows {
		result[i] = nativeClassArrivalException(row)
	}
	return result, nil
}

func classArrivalKeys(classes []string) []string {
	keys := make([]string, 0, len(classes))
	for _, class := range classes {
		if key := strings.ToLower(strings.TrimSpace(class)); key != "" {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}
