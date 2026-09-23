package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// SetPortalConsent writes the photo slice the parents portal decided on. It
// does not consult the photo feature switch: a withdrawal must land even
// while photos are disabled, and a grant only records the consent. The caller
// holds the child's row lock, so the slice it read is the one it overwrites.
func (s *StudentPhotoService) SetPortalConsent(ctx context.Context, photo domain.StudentPhoto) error {
	if photo.StudentID <= 0 || (photo.PhotoConsentGivenAt == nil) != (photo.PhotoConsentGivenBy == nil) {
		return &domain.InvalidStudentRecordError{Reason: "student ID is required and photo consent time and actor go together"}
	}
	return observeRun(ctx, s.observe, "set_student_photo_consent", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		writeStats, err := s.store.SavePhoto(txCtx, photo)
		stats.Add(writeStats)
		return err
	})
}
