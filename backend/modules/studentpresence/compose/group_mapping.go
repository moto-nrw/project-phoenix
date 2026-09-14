package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) ListGroupMappings(ctx context.Context, filter studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error) {
	rows, err := e.Service.ListGroupMappings(ctx, ports.GroupMappingFilter(filter))
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.GroupMapping, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.GroupMapping(row))
	}
	return result, nil
}

func (e engine) RecordGroupMapping(ctx context.Context, combinedID, groupID int64) (studentpresence.GroupMapping, error) {
	row, err := e.Service.RecordGroupMapping(ctx, combinedID, groupID)
	return studentpresence.GroupMapping(row), err
}
