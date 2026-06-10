// backend/database/repositories/users/privacy_consent.go
package users

import (
	"context"
	"encoding/json"
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

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: err,
		}
	}

	return consents, nil
}

// FindByStudentIDAndPolicyVersion retrieves a privacy consent for a student and policy version
func (r *PrivacyConsentRepository) FindByStudentIDAndPolicyVersion(ctx context.Context, studentID int64, policyVersion string) (*users.PrivacyConsent, error) {
	consent := new(users.PrivacyConsent)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(consent).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".student_id = ? AND "privacy_consent".policy_version = ?`, studentID, policyVersion)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID and policy version",
			Err: err,
		}
	}

	return consent, nil
}

// FindActiveByStudentID retrieves active privacy consents for a student
func (r *PrivacyConsentRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	now := time.Now()

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&consents).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".student_id = ? AND "privacy_consent".accepted = TRUE AND ("privacy_consent".expires_at IS NULL OR "privacy_consent".expires_at > ?)`, studentID, now)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by student ID",
			Err: err,
		}
	}

	return consents, nil
}

// FindExpired retrieves all expired privacy consents
func (r *PrivacyConsentRepository) FindExpired(ctx context.Context) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	now := time.Now()

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&consents).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".expires_at < ? AND "privacy_consent".accepted = TRUE`, now)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find expired",
			Err: err,
		}
	}

	return consents, nil
}

// FindNeedingRenewal retrieves all privacy consents that need renewal
func (r *PrivacyConsentRepository) FindNeedingRenewal(ctx context.Context) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&consents).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Where(`"privacy_consent".renewal_required = TRUE AND "privacy_consent".accepted = TRUE`)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find needing renewal",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx, &settings); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list accepted retention settings",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "accept",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "revoke",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "revoke privacy_consent")
}

// SetExpiryDate sets the expiry date for a privacy consent
func (r *PrivacyConsentRepository) SetExpiryDate(ctx context.Context, id int64, expiresAt time.Time) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Set("expires_at = ?", expiresAt).
		Where(`"privacy_consent".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set expiry date",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "set expiry date privacy_consent")
}

// SetRenewalRequired sets whether renewal is required for a privacy consent
func (r *PrivacyConsentRepository) SetRenewalRequired(ctx context.Context, id int64, renewalRequired bool) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Set("renewal_required = ?", renewalRequired).
		Where(`"privacy_consent".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set renewal required",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "set renewal required privacy_consent")
}

// UpdateDetails updates the details for a privacy consent
func (r *PrivacyConsentRepository) UpdateDetails(ctx context.Context, id int64, details string) error {
	// Parse the JSON string to ensure it's valid
	var detailsMap map[string]interface{}
	if err := json.Unmarshal([]byte(details), &detailsMap); err != nil {
		return fmt.Errorf("invalid details JSON format: %w", err)
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Set("details = ?", details).
		Where(`"privacy_consent".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update details",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update details privacy_consent")
}

// Create overrides the base Create method to handle validation
func (r *PrivacyConsentRepository) Create(ctx context.Context, consent *users.PrivacyConsent) error {
	if consent == nil {
		return fmt.Errorf("privacy consent cannot be nil")
	}

	// Validate consent
	if err := consent.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, consent)
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

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
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

// ListWithOptions provides a type-safe way to list privacy consents with query options
func (r *PrivacyConsentRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&consents).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return consents, nil
}

// FindWithStudent retrieves a privacy consent with its associated student
func (r *PrivacyConsentRepository) FindWithStudent(ctx context.Context, id int64) (*users.PrivacyConsent, error) {
	consent := new(users.PrivacyConsent)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(consent).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Relation("Student").
		Where(`"privacy_consent".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student",
			Err: err,
		}
	}

	return consent, nil
}

// FindWithStudentAndPerson retrieves a privacy consent with its associated student and person
func (r *PrivacyConsentRepository) FindWithStudentAndPerson(ctx context.Context, id int64) (*users.PrivacyConsent, error) {
	consent := new(users.PrivacyConsent)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(consent).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Relation("Student").
		Relation("Student.Person").
		Where(`"privacy_consent".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student and person",
			Err: err,
		}
	}

	return consent, nil
}

// MarkAutoRenewals identifies consents approaching expiration and sets their renewal_required flag
func (r *PrivacyConsentRepository) MarkAutoRenewals(ctx context.Context, daysBeforeExpiry int) (int, error) {
	// Calculate the date threshold for renewals
	thresholdDate := time.Now().AddDate(0, 0, daysBeforeExpiry)

	// Set renewal_required for all consents approaching expiration
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		ModelTableExpr(`users.privacy_consents AS "privacy_consent"`).
		Set("renewal_required = TRUE").
		Where(`"privacy_consent".accepted = TRUE`).
		Where(`"privacy_consent".expires_at IS NOT NULL AND "privacy_consent".expires_at <= ?`, thresholdDate).
		Where(`"privacy_consent".renewal_required = FALSE`)

	if where, val, ok := base.TenantWhere(ctx, "privacy_consent"); ok {
		query = query.Where(where, val)
	}

	res, err := query.Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "mark auto renewals",
			Err: err,
		}
	}

	// Get the number of affected rows
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count affected rows",
			Err: err,
		}
	}

	return int(affected), nil
}
