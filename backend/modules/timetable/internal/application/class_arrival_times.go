package application

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

type classArrivalStore interface {
	ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, domain.OperationStats, error)
}

type ClassArrivalQueries struct {
	store   classArrivalStore
	observe ports.Observer
}

func NewClassArrivalQueries(store classArrivalStore, observe ports.Observer) *ClassArrivalQueries {
	return &ClassArrivalQueries{store: store, observe: observe}
}

func (q *ClassArrivalQueries) ListClassArrivalTimes(ctx context.Context, classes []string) (map[string]map[string]string, error) {
	keys := make([]string, 0, len(classes))
	for _, class := range classes {
		if key := strings.ToLower(strings.TrimSpace(class)); key != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return map[string]map[string]string{}, nil
	}
	slices.Sort(keys)
	started := time.Now()
	times, stats, err := q.store.ListClassArrivalTimes(ctx, slices.Compact(keys))
	q.observe(ports.Observation{Operation: "list_class_arrival_times", Duration: time.Since(started), Stats: stats, Err: err})
	return times, err
}
