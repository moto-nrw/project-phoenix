// backend/database/repositories/users/privacy_consent.go
package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// PrivacyConsentRepository implements users.PrivacyConsentRepository interface
type PrivacyConsentRepository struct {
	*base.Repository[*users.PrivacyConsent]
	db *bun.DB
}

// NewPrivacyConsentRepository creates a new PrivacyConsentRepository
func NewPrivacyConsentRepository(db *bun.DB) users.PrivacyConsentRepository {
	repo := base.NewRepository[*users.PrivacyConsent](db, "users.privacy_consents", "PrivacyConsent")
	repo.TenantScoped = true
	return &PrivacyConsentRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStudentID retrieves privacy consents for a student
func (r *PrivacyConsentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&consents).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".student_id = ?`, studentID)

	query = base.WithTenantFilter(ctx, query, "privacy_consent")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return consents, nil
}

// FindActiveByStudentID retrieves active privacy consents for a student
func (r *PrivacyConsentRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	now := time.Now()

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&consents).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".student_id = ? AND "privacy_consent".accepted = TRUE AND ("privacy_consent".expires_at IS NULL OR "privacy_consent".expires_at > ?)`, studentID, now)

	query = base.WithTenantFilter(ctx, query, "privacy_consent")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by student ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return consents, nil
}

// ListAcceptedRetentionSettings returns the distinct (student_id,
// data_retention_days) pairs of accepted privacy consents, ordered by
// student_id. Custom method: the DISTINCT projection is not expressible via
// the generic List shape. Feeds the GDPR visit-cleanup worklist.
func (r *PrivacyConsentRepository) ListAcceptedRetentionSettings(ctx context.Context) ([]users.StudentRetentionSetting, error) {
	var settings []users.StudentRetentionSetting

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.privacy_consents AS "privacy_consent"`).
		ColumnExpr(`DISTINCT "privacy_consent".student_id, "privacy_consent".data_retention_days`).
		Where(`"privacy_consent".accepted = TRUE`).
		OrderExpr(`"privacy_consent".student_id`)

	query = base.WithTenantFilter(ctx, query, "privacy_consent")

	if err := query.Scan(ctx, &settings); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list accepted retention settings",
			Err: base.TranslateNotFound(err),
		}
	}

	return settings, nil
}

// Accept marks a privacy consent as accepted
func (r *PrivacyConsentRepository) Accept(ctx context.Context, id int64, acceptedAt time.Time) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Set("accepted = TRUE").
		Set("accepted_at = ?", acceptedAt).
		Where(`"privacy_consent".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "privacy_consent")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "accept",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "accept privacy_consent")
}

// Revoke revokes a privacy consent
func (r *PrivacyConsentRepository) Revoke(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Set("accepted = FALSE").
		Where(`"privacy_consent".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "privacy_consent")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "revoke",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "revoke privacy_consent")
}

// Update overrides the base Update method to handle validation
func (r *PrivacyConsentRepository) Update(ctx context.Context, consent *users.PrivacyConsent) error {
	if consent == nil {
		return fmt.Errorf("privacy consent cannot be nil")
	}

	// Validate consent
	if err := consent.Validate(); err != nil {
		return err
	}

	// Execute update with correct table alias
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(consent).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		WherePK()

	query = base.WithTenantFilter(ctx, query, "privacy_consent")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update privacy_consent")
}

// Legacy method to maintain compatibility with old interface
func (r *PrivacyConsentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.PrivacyConsent, error) {
	options := modelBase.NewQueryOptions()
	options.Filter = buildPrivacyConsentFilter(filters)
	return r.ListWithOptions(ctx, options)
}

// buildPrivacyConsentFilter converts legacy filter map to QueryOptions filter
func buildPrivacyConsentFilter(filters map[string]interface{}) *modelBase.Filter {
	filter := modelBase.NewFilter().WithTableAlias("privacy_consent")

	for field, value := range filters {
		if value == nil {
			continue
		}
		applyPrivacyConsentFilterField(filter, field, value)
	}

	return filter
}

// applyPrivacyConsentFilterField applies a single filter field
func applyPrivacyConsentFilterField(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "accepted", "renewal_required", "policy_version":
		filter.Equal(field, value)
	case "active":
		if boolValue, ok := value.(bool); ok && boolValue {
			now := time.Now()
			filter.Equal("accepted", true)
			orFilter := modelBase.NewFilter().WithTableAlias("privacy_consent")
			orFilter.IsNull("expires_at")
			orFilter2 := modelBase.NewFilter().WithTableAlias("privacy_consent")
			orFilter2.GreaterThan("expires_at", now)
			filter.Or(*orFilter).Or(*orFilter2)
		}
	case "expired":
		if boolValue, ok := value.(bool); ok && boolValue {
			filter.LessThan("expires_at", time.Now())
		}
	default:
		filter.Equal(field, value)
	}
}
