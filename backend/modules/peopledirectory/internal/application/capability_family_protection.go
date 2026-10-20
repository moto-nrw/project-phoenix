package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) CurrentFamilyProtection(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	for _, id := range studentIDs {
		if id <= 0 {
			return nil, &peopledirectory.InvalidStudentError{Reason: "family protection requires positive student IDs"}
		}
	}
	if len(studentIDs) == 0 {
		return map[int64]bool{}, nil
	}

	return s.students.CurrentFamilyProtection(ctx, peopledirectory.PositiveIDs(studentIDs))
}

func (s *Service) SetFamilyProtection(ctx context.Context, input peopledirectory.SetFamilyProtection) (bool, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.StudentID <= 0 || input.ActorAccountID <= 0 || input.Reason == "" ||
		len([]rune(input.Reason)) > peopledirectory.MaxFamilyProtectionReasonRunes {
		return false, peopledirectory.ErrFamilyProtectionInvalid
	}

	enabled, err := s.students.SetFamilyProtection(ctx, domain.FamilyProtectionChange{
		StudentID: input.StudentID, Enabled: input.Enabled,
		Reason: input.Reason, ActorAccountID: input.ActorAccountID,
	})
	return enabled, MapError(err)
}
