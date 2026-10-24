package users

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

func nativeParentRequestEvent(row *models.ParentRequestEvent) (*directory.ParentRequestEventSnapshot, error) {
	if row == nil {
		return nil, nil
	}
	value := &directory.ParentRequestEventSnapshot{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID, StudentID: row.StudentID, RequestType: row.RequestType, RequestID: row.RequestID, EventType: row.EventType, ActorAccountID: row.ActorAccountID, Version: row.Version}
	if row.Payload != nil {
		raw, err := json.Marshal(row.Payload)
		if err != nil {
			return nil, err
		}
		value.Payload = raw
	}
	return value, nil
}
func retainedParentRequestEvent(row *directory.ParentRequestEventSnapshot) (*models.ParentRequestEvent, error) {
	if row == nil {
		return nil, nil
	}
	value := &models.ParentRequestEvent{StudentID: row.StudentID, RequestType: row.RequestType, RequestID: row.RequestID, EventType: row.EventType, ActorAccountID: row.ActorAccountID, Version: row.Version}
	value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt = row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt
	if row.Payload != nil {
		decoder := json.NewDecoder(bytes.NewReader(row.Payload))
		decoder.UseNumber()
		if err := decoder.Decode(&value.Payload); err != nil {
			return nil, err
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("parent request event payload contains more than one JSON value")
		}
	}
	return value, nil
}
func (r *ParentRequestEventRepository) AppendParentRequestEvent(ctx context.Context, value *directory.ParentRequestEventSnapshot) error {
	row, err := retainedParentRequestEvent(value)
	if err != nil {
		return err
	}
	// Invoke the exact original insert, including tenant binding, nil payload
	// materialization, clock_timestamp and its database error wrapper.
	err = r.Create(ctx, row)
	if value != nil {
		mapped, mapErr := nativeParentRequestEvent(row)
		if mapErr != nil {
			return mapErr
		}
		*value = *mapped
	}
	return err
}
func (r *ParentRequestEventRepository) ParentRequestEventsForRequest(ctx context.Context, kind string, id int64) ([]*directory.ParentRequestEventSnapshot, error) {
	rows, err := r.ListForRequest(ctx, kind, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*directory.ParentRequestEventSnapshot, len(rows))
	for i, row := range rows {
		value, mapErr := nativeParentRequestEvent(row)
		if mapErr != nil {
			return nil, mapErr
		}
		result[i] = value
	}
	return result, err
}
