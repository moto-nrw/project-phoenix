package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) ListCombinedGroups(ctx context.Context, filter studentpresence.CombinedGroupFilter) ([]studentpresence.CombinedGroup, error) {
	rows, err := e.Service.ListCombinedGroups(ctx, ports.CombinedGroupFilter(filter))
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.CombinedGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.CombinedGroup(row))
	}
	return result, nil
}

func (e engine) GetCombinedGroup(ctx context.Context, id int64) (studentpresence.CombinedGroup, error) {
	row, err := e.Service.GetCombinedGroup(ctx, id)
	if errors.Is(err, ports.ErrCombinedGroupNotFound) {
		return studentpresence.CombinedGroup{}, studentpresence.ErrCombinedGroupNotFound
	}
	return studentpresence.CombinedGroup(row), err
}
func (e engine) RecordCombination(ctx context.Context, start time.Time, end *time.Time) (studentpresence.CombinedGroup, error) {
	row, err := e.Service.RecordCombination(ctx, start, end)
	return studentpresence.CombinedGroup(row), err
}
func (e engine) ReviseCombination(ctx context.Context, id int64, start time.Time, end *time.Time) (studentpresence.CombinedGroup, error) {
	row, err := e.Service.ReviseCombination(ctx, id, start, end)
	return studentpresence.CombinedGroup(row), err
}
