package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error) {
	if studentID <= 0 {
		return "", invalidStudent("student ID is required")
	}

	storedURL, err := s.studentPhotos.FindPhoto(ctx, studentID, filename)
	return storedURL, mapPhotoError(err)
}

func (s *Service) CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error {
	if studentID <= 0 {
		return invalidStudent("student ID is required")
	}
	if storedURL == "" {
		return invalidStudent("a stored photo URL is required")
	}

	return mapPhotoError(s.studentPhotos.CommitPhoto(ctx, studentID, storedURL, consentAck))
}

func (s *Service) ClearStudentPhoto(ctx context.Context, studentID int64) (string, error) {
	if studentID <= 0 {
		return "", invalidStudent("student ID is required")
	}

	clearedURL, err := s.studentPhotos.ClearPhoto(ctx, studentID)
	return clearedURL, mapPhotoError(err)
}

func (s *Service) PurgeStudentPhotos(ctx context.Context) ([]string, error) {
	urls, err := s.studentPhotos.PurgePhotos(ctx)
	return urls, mapPhotoError(err)
}

func (s *Service) ApplyStudentPhotoConsent(
	ctx context.Context,
	current peopledirectory.StudentPhotoState,
	requestedConsent *bool,
) (peopledirectory.StudentPhotoState, error) {
	updated, err := s.studentPhotos.ApplyPhotoConsent(ctx, domain.StudentPhoto{
		StudentID:           current.StudentID,
		PhotoPath:           current.PhotoPath,
		PhotoConsentGivenAt: current.PhotoConsentGivenAt,
		PhotoConsentGivenBy: current.PhotoConsentGivenBy,
	}, requestedConsent)
	if err != nil {
		return current, mapPhotoError(err)
	}
	return peopledirectory.StudentPhotoState{
		StudentID:           updated.StudentID,
		PhotoPath:           updated.PhotoPath,
		PhotoConsentGivenAt: updated.PhotoConsentGivenAt,
		PhotoConsentGivenBy: updated.PhotoConsentGivenBy,
	}, nil
}

func (s *Service) ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string) error {
	return mapPhotoError(s.studentPhotos.ScheduleUnlink(ctx, storedURL))
}
