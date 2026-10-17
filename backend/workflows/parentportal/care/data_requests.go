package care

import (
	"context"
	"encoding/json"
	"errors"
)

// createDataRequest records the request through the composed owner command,
// preserving the complete returned snapshot and the owner's error cause.
func (s *Service) createDataRequest(ctx context.Context, row *StudentDataChangeRequest) error {
	if s.DataRequests == nil {
		return errors.New("parent: student data request command is not configured")
	}
	created, err := s.DataRequests.CreateStudentDataRequest(ctx, *row)
	if err != nil {
		return err
	}
	*row = created
	return nil
}
func (s *Service) updatePendingDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	if s.DataRequests == nil {
		return errors.New("parent: student data request command is not configured")
	}
	return s.DataRequests.UpdatePendingStudentDataRequest(ctx, id, value)
}
