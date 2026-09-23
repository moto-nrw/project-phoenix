package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// guardianEmailIndex is the per-school, case-insensitive unique index on
// users.guardian_profiles (tenant_id, LOWER(email)).
const guardianEmailIndex = "idx_guardian_profiles_tenant_email"

// tenantDB resolves the caller's transaction and refuses without a tenant:
// every parents-portal write belongs to exactly one school.
func (s *GuardianStore) tenantDB(ctx context.Context) (bun.IDB, int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if tenantID <= 0 {
		return nil, 0, domain.ErrTenantRequired
	}
	return db, tenantID, nil
}

// exec runs one write statement and reports its rows affected.
func exec(ctx context.Context, operation string, run func(context.Context, ...any) (sql.Result, error)) (int64, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := run(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, wrapGuardianWriteError(operation, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: %s: %w", operation, err)
	}
	stats.Rows = affected
	return affected, stats, nil
}

// wrapGuardianWriteError keeps the database error in the chain and marks the
// e-mail unique violation so callers can tell it apart.
func wrapGuardianWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, guardianEmailIndex) {
		return fmt.Errorf("people directory postgres: %s: %w: %w", operation, domain.ErrGuardianEmailTaken, err)
	}
	return fmt.Errorf("people directory postgres: %s: %w", operation, err)
}

type guardianContactRow struct {
	bun.BaseModel          `bun:"table:guardian_profiles"`
	ID                     int64   `bun:"id,pk,autoincrement"`
	TenantID               int64   `bun:"tenant_id,notnull"`
	FirstName              string  `bun:"first_name"`
	LastName               string  `bun:"last_name"`
	Email                  *string `bun:"email"`
	AddressStreet          *string `bun:"address_street"`
	AddressCity            *string `bun:"address_city"`
	AddressPostalCode      *string `bun:"address_postal_code"`
	PreferredContactMethod string  `bun:"preferred_contact_method"`
	LanguagePreference     string  `bun:"language_preference"`
}

// guardianContactColumns are the columns a contact write sets. Everything
// else on the profile (account link, notes, portal locale) stays as stored.
var guardianContactColumns = []string{
	"first_name", "last_name", "email", "address_street", "address_city",
	"address_postal_code", "preferred_contact_method", "language_preference",
}

func newGuardianContactRow(contact domain.GuardianContact, tenantID int64) *guardianContactRow {
	return &guardianContactRow{
		TenantID: tenantID, FirstName: contact.FirstName, LastName: contact.LastName, Email: contact.Email,
		AddressStreet: contact.AddressStreet, AddressCity: contact.AddressCity, AddressPostalCode: contact.PostalCode,
		PreferredContactMethod: contact.PreferredContactMethod, LanguagePreference: contact.LanguagePreference,
	}
}

// InsertContact inserts a profile without an account; has_account, the
// timestamps and the ID are the database's defaults.
func (s *GuardianStore) InsertContact(ctx context.Context, contact domain.GuardianContact) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	row := newGuardianContactRow(contact, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(row).ModelTableExpr("users.guardian_profiles").
		Column(append([]string{"tenant_id"}, guardianContactColumns...)...).
		Returning("id").Scan(ctx, &row.ID)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, wrapGuardianWriteError("create guardian contact", err)
	}
	stats.Rows = 1
	return row.ID, stats, nil
}

// UpdateContact rewrites the contact columns of one profile; updated_at is
// moved by the table's trigger.
func (s *GuardianStore) UpdateContact(ctx context.Context, guardianID int64, contact domain.GuardianContact) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	row := newGuardianContactRow(contact, tenantID)
	row.ID = guardianID
	affected, stats, err := exec(ctx, "update guardian contact", db.NewUpdate().Model(row).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Column(guardianContactColumns...).
		Where(`"guardian_profile".id = ?`, guardianID).
		Where(`"guardian_profile".tenant_id = ?`, tenantID).Exec)
	return affected > 0, stats, err
}

func (s *GuardianStore) DeletePhones(ctx context.Context, guardianID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	_, stats, err := exec(ctx, "delete guardian phones", db.NewDelete().
		TableExpr("users.guardian_phone_numbers").
		Where("guardian_profile_id = ?", guardianID).
		Where("tenant_id = ?", tenantID).Exec)
	return stats, err
}

type guardianPhoneRow struct {
	bun.BaseModel     `bun:"table:guardian_phone_numbers"`
	ID                int64   `bun:"id,pk,autoincrement"`
	TenantID          int64   `bun:"tenant_id,notnull"`
	GuardianProfileID int64   `bun:"guardian_profile_id,notnull"`
	PhoneNumber       string  `bun:"phone_number,notnull"`
	PhoneType         string  `bun:"phone_type,notnull"`
	Label             *string `bun:"label"`
	IsPrimary         bool    `bun:"is_primary,notnull"`
	Priority          int     `bun:"priority,notnull"`
}

// InsertPhone adds one phone row of the profile. The composite foreign key
// (tenant_id, guardian_profile_id) rejects a profile of another school.
func (s *GuardianStore) InsertPhone(ctx context.Context, guardianID int64, phone domain.GuardianPhoneRecord) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	row := &guardianPhoneRow{
		TenantID: tenantID, GuardianProfileID: guardianID, PhoneNumber: phone.PhoneNumber,
		PhoneType: phone.PhoneType, Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(row).ModelTableExpr("users.guardian_phone_numbers").
		Column("tenant_id", "guardian_profile_id", "phone_number", "phone_type", "label", "is_primary", "priority").
		Returning("id").Scan(ctx, &row.ID)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, wrapGuardianWriteError("add guardian phone", err)
	}
	stats.Rows = 1
	return row.ID, stats, nil
}

func (s *GuardianStore) UpdatePhoneNumber(ctx context.Context, phoneID int64, number string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "update guardian phone number", db.NewUpdate().
		TableExpr("users.guardian_phone_numbers").
		Set("phone_number = ?", number).
		Where("id = ?", phoneID).
		Where("tenant_id = ?", tenantID).Exec)
	return affected > 0, stats, err
}

func (s *GuardianStore) DeletePhone(ctx context.Context, phoneID int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "delete guardian phone", db.NewDelete().
		TableExpr("users.guardian_phone_numbers").
		Where("id = ?", phoneID).
		Where("tenant_id = ?", tenantID).Exec)
	return affected > 0, stats, err
}

// InsertLinkIfAbsent inserts the relationship unless the (tenant, student,
// guardian) pair exists already, and returns the new ID (0 when it existed).
// The composite foreign keys reject rows of another school. A primary link
// first demotes the child's other primary: the relationship table keeps one
// per child with a partial unique index instead of the old demotion trigger.
func (s *GuardianStore) InsertLinkIfAbsent(ctx context.Context, link domain.GuardianLinkRecord) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	if link.IsPrimary {
		stats.Queries++
		if _, err := db.NewRaw(`UPDATE users.student_guardian_relationships SET is_primary = FALSE
			WHERE tenant_id = ? AND student_id = ? AND is_primary`, tenantID, link.StudentID).Exec(ctx); err != nil {
			stats.StatementDuration = time.Since(started)
			return 0, stats, wrapGuardianWriteError("demote primary guardian", err)
		}
	}
	var ids []int64
	stats.Queries++
	err = db.NewRaw(`INSERT INTO users.student_guardian_relationships
		(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
		 is_primary, is_emergency_contact, emergency_priority, is_payer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, student_id, guardian_profile_id) DO NOTHING
		RETURNING id`,
		tenantID, link.StudentID, link.GuardianProfileID, link.RelationshipType, link.GuardianRole,
		link.IsPrimary, link.IsEmergencyContact, link.EmergencyPriority, link.IsPayer).Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, wrapGuardianWriteError("link guardian contact", err)
	}
	if len(ids) == 0 {
		// DO NOTHING returned no row: the pair is linked already.
		return 0, stats, nil
	}
	stats.Rows = 1
	return ids[0], stats, nil
}

// LockLinkForPickup takes the relationship row lock the pickup permission is
// written under, sets the emergency contact flag when one is supplied, and
// reports whether the tenant has the relationship.
func (s *GuardianStore) LockLinkForPickup(ctx context.Context, linkID int64, isEmergencyContact *bool) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if isEmergencyContact != nil {
		affected, stats, err := exec(ctx, "patch guardian link emergency contact", db.NewUpdate().
			TableExpr("users.student_guardian_relationships").
			Set("is_emergency_contact = ?", *isEmergencyContact).
			Where("id = ?", linkID).
			Where("tenant_id = ?", tenantID).Exec)
		return affected > 0, stats, err
	}
	var ids []int64
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr("users.student_guardian_relationships").
		Column("id").
		Where("id = ?", linkID).
		Where("tenant_id = ?", tenantID).
		For("UPDATE").Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: lock guardian link: %w", err)
	}
	stats.Rows = int64(len(ids))
	return len(ids) > 0, stats, nil
}

// GuardianAccount reads the portal account the guardian profile names; a new
// link's access row binds it.
func (s *GuardianStore) GuardianAccount(ctx context.Context, guardianID int64) (*int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		AccountID *int64 `bun:"account_id"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr("users.guardian_profiles").
		Column("account_id").
		Where("id = ?", guardianID).
		Where("tenant_id = ?", tenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: read guardian account: %w", err)
	}
	if len(rows) == 0 {
		return nil, stats, nil
	}
	stats.Rows = 1
	return rows[0].AccountID, stats, nil
}

// SetPortalLocale updates the account's profiles of the tenant in context.
func (s *GuardianStore) SetPortalLocale(ctx context.Context, accountID int64, locale string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	return exec(ctx, "set guardian portal locale", db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", locale).
		Set("updated_at = NOW()").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).Exec)
}
