package application

import (
	"context"
	"time"
)

func (s *Service) AnonymizeRetiredStaffPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	if personID <= 0 {
		return false, invalid("person ID is required")
	}
	if updatedAt.IsZero() {
		return false, invalid("read person revision is required")
	}

	anonymized, err := s.AnonymizeIfUnchanged(ctx, personID, updatedAt)
	return anonymized, MapError(err)
}
