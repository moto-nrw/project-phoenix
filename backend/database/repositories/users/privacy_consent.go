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
	return &PrivacyConsentRepository{
		Repository: base.NewRepository[*users.PrivacyConsent](db, "users.privacy_consents", "PrivacyConsent"),
		db:         db,
	}
}

// FindByStudentID retrieves privacy consents for a student
func (r *PrivacyConsentRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	err := r.db.NewSelect().
		Model(&consents).
		Where("student_id = ?", studentID).
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(consent).
		Where("student_id = ? AND policy_version = ?", studentID, policyVersion).
		Scan(ctx)

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

	err := r.db.NewSelect().
		Model(&consents).
		Where("student_id = ? AND accepted = TRUE AND (expires_at IS NULL OR expires_at > ?)", studentID, now).
		Scan(ctx)

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

	err := r.db.NewSelect().
		Model(&consents).
		Where("expires_at < ? AND accepted = TRUE", now).
		Scan(ctx)

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

	err := r.db.NewSelect().
		Model(&consents).
		Where("renewal_required = TRUE AND accepted = TRUE").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find needing renewal",
			Err: err,
		}
	}

	return consents, nil
}

// Accept marks a privacy consent as accepted
func (r *PrivacyConsentRepository) Accept(ctx context.Context, id int64, acceptedAt time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		Set("accepted = TRUE").
		Set("accepted_at = ?", acceptedAt).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "accept",
			Err: err,
		}
	}

	return nil
}

// Revoke revokes a privacy consent
func (r *PrivacyConsentRepository) Revoke(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		Set("accepted = FALSE").
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "revoke",
			Err: err,
		}
	}

	return nil
}

// SetExpiryDate sets the expiry date for a privacy consent
func (r *PrivacyConsentRepository) SetExpiryDate(ctx context.Context, id int64, expiresAt time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		Set("expires_at = ?", expiresAt).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set expiry date",
			Err: err,
		}
	}

	return nil
}

// SetRenewalRequired sets whether renewal is required for a privacy consent
func (r *PrivacyConsentRepository) SetRenewalRequired(ctx context.Context, id int64, renewalRequired bool) error {
	_, err := r.db.NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		Set("renewal_required = ?", renewalRequired).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set renewal required",
			Err: err,
		}
	}

	return nil
}

// UpdateDetails updates the details for a privacy consent
func (r *PrivacyConsentRepository) UpdateDetails(ctx context.Context, id int64, details string) error {
	// Parse the JSON string to ensure it's valid
	var detailsMap map[string]interface{}
	if err := json.Unmarshal([]byte(details), &detailsMap); err != nil {
		return fmt.Errorf("invalid details JSON format: %w", err)
	}

	_, err := r.db.NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		Set("details = ?", details).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update details",
			Err: err,
		}
	}

	return nil
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

	// Use the base Update method
	return r.Repository.Update(ctx, consent)
}

// Legacy method to maintain compatibility with old interface
func (r *PrivacyConsentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.PrivacyConsent, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "accepted":
				filter.Equal("accepted", value)
			case "renewal_required":
				filter.Equal("renewal_required", value)
			case "active":
				if boolValue, ok := value.(bool); ok && boolValue {
					now := time.Now()
					filter.Equal("accepted", true)
					filter.Where("expires_at IS NULL OR expires_at > ?", modelBase.OpEqual, now)
				}
			case "expired":
				if boolValue, ok := value.(bool); ok && boolValue {
					now := time.Now()
					filter.Where("expires_at < ?", modelBase.OpLessThan, now)
				}
			case "policy_version":
				filter.Equal("policy_version", value)
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list privacy consents with query options
func (r *PrivacyConsentRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.PrivacyConsent, error) {
	var consents []*users.PrivacyConsent
	query := r.db.NewSelect().Model(&consents)

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
	err := r.db.NewSelect().
		Model(consent).
		Relation("Student").
		Where("id = ?", id).
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(consent).
		Relation("Student").
		Relation("Student.Person").
		Where("id = ?", id).
		Scan(ctx)

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
	res, err := r.db.NewUpdate().
		Model((*users.PrivacyConsent)(nil)).
		Set("renewal_required = TRUE").
		Where("accepted = TRUE").
		Where("expires_at IS NOT NULL AND expires_at <= ?", thresholdDate).
		Where("renewal_required = FALSE").
		Exec(ctx)

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
