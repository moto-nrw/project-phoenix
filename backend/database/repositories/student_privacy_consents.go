package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type studentPrivacyConsents struct {
	presence interface {
		ListPrivacyConsents(context.Context, int64) ([]studentpresence.PrivacyConsent, error)
		RecordPrivacyConsent(context.Context, studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error)
		RevisePrivacyConsent(context.Context, studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error)
	}
}

func consentFromPresence(value studentpresence.PrivacyConsent) (*users.PrivacyConsent, error) {
	var details map[string]any
	if len(value.Details) > 0 {
		if err := json.Unmarshal(value.Details, &details); err != nil {
			return nil, err
		}
	}
	row := &users.PrivacyConsent{
		StudentID: value.StudentID, PolicyVersion: value.PolicyVersion, Accepted: value.Accepted,
		AcceptedAt: value.AcceptedAt, ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays,
		RenewalRequired: value.RenewalRequired, DataRetentionDays: value.DataRetentionDays, Details: details,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	row.SetTenantID(value.TenantID)
	return row, nil
}

func (s studentPrivacyConsents) FindByStudentID(ctx context.Context, id int64) ([]*users.PrivacyConsent, error) {
	rows, err := s.presence.ListPrivacyConsents(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]*users.PrivacyConsent, 0, len(rows))
	for _, row := range rows {
		consent, err := consentFromPresence(row)
		if err != nil {
			return nil, err
		}
		result = append(result, consent)
	}
	return result, nil
}
func (s studentPrivacyConsents) Create(ctx context.Context, value *users.PrivacyConsent) error {
	return s.write(ctx, value, false)
}
func (s studentPrivacyConsents) Update(ctx context.Context, value *users.PrivacyConsent) error {
	return s.write(ctx, value, true)
}
func (s studentPrivacyConsents) write(ctx context.Context, value *users.PrivacyConsent, revise bool) error {
	if value == nil {
		return errors.New("privacy consent cannot be nil")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	details, err := json.Marshal(value.Details)
	if err != nil {
		return err
	}
	input := studentpresence.PrivacyConsent{ID: value.ID, StudentID: value.StudentID,
		PolicyVersion: value.PolicyVersion, Accepted: value.Accepted, AcceptedAt: value.AcceptedAt,
		ExpiresAt: value.ExpiresAt, DurationDays: value.DurationDays, RenewalRequired: value.RenewalRequired,
		DataRetentionDays: value.DataRetentionDays, Details: details}
	var result studentpresence.PrivacyConsent
	if revise {
		result, err = s.presence.RevisePrivacyConsent(ctx, input)
	} else {
		result, err = s.presence.RecordPrivacyConsent(ctx, input)
	}
	if err != nil {
		return err
	}
	consent, err := consentFromPresence(result)
	if err != nil {
		return err
	}
	*value = *consent
	return nil
}

func NewStudentPrivacyConsentStore(db *bun.DB) *studentPrivacyConsents {
	return StudentPrivacyConsentCapability(newStudentPresence(db))
}
func StudentPrivacyConsentCapability(presence *studentpresence.Module) *studentPrivacyConsents {
	return &studentPrivacyConsents{presence: presence}
}
