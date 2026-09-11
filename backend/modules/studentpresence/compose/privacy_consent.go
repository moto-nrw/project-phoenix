package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func privacyConsentToPublic(row ports.PrivacyConsent) studentpresence.PrivacyConsent {
	return studentpresence.PrivacyConsent{
		ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PolicyVersion: row.PolicyVersion, Accepted: row.Accepted, AcceptedAt: row.AcceptedAt, ExpiresAt: row.ExpiresAt,
		DurationDays: row.DurationDays, RenewalRequired: row.RenewalRequired, DataRetentionDays: row.DataRetentionDays,
	}
}

func mapPrivacyConsentError(err error) error {
	if errors.Is(err, ports.ErrInvalidPrivacyConsent) {
		return studentpresence.ErrInvalidPrivacyConsent
	}
	return err
}

func (e engine) ListPrivacyConsents(ctx context.Context, studentID int64) ([]studentpresence.PrivacyConsent, error) {
	rows, err := e.Service.ListPrivacyConsents(ctx, studentID)
	if err != nil {
		return nil, mapPrivacyConsentError(err)
	}
	result := make([]studentpresence.PrivacyConsent, 0, len(rows))
	for _, row := range rows {
		result = append(result, privacyConsentToPublic(row))
	}
	return result, nil
}

func (e engine) RecordPrivacyConsent(ctx context.Context, value studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error) {
	row := ports.PrivacyConsent{
		StudentID: value.StudentID, PolicyVersion: value.PolicyVersion, Accepted: value.Accepted,
		AcceptedAt: value.AcceptedAt, ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays,
		RenewalRequired: value.RenewalRequired, DataRetentionDays: value.DataRetentionDays,
	}
	if err := e.Service.RecordPrivacyConsent(ctx, &row); err != nil {
		return studentpresence.PrivacyConsent{}, mapPrivacyConsentError(err)
	}
	return privacyConsentToPublic(row), nil
}
