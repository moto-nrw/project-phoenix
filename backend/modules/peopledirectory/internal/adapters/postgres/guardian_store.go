package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

type guardianRow struct {
	bun.BaseModel          `bun:"table:guardian_profiles,alias:guardian_profile"`
	ID                     int64     `bun:"id,pk,autoincrement"`
	CreatedAt              time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt              time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID               int64     `bun:"tenant_id,notnull"`
	FirstName              string    `bun:"first_name"`
	LastName               string    `bun:"last_name"`
	Email                  *string   `bun:"email"`
	AddressStreet          *string   `bun:"address_street"`
	AddressCity            *string   `bun:"address_city"`
	AddressPostalCode      *string   `bun:"address_postal_code"`
	PreferredContactMethod string    `bun:"preferred_contact_method"`
	LanguagePreference     string    `bun:"language_preference"`
	Notes                  *string   `bun:"notes"`
	HasAccount             bool      `bun:"has_account"`
	AccountID              *int64    `bun:"account_id"`
}

type guardianLinkRow struct {
	bun.BaseModel      `bun:"alias:student_guardian"`
	ID                 int64          `bun:"id,pk,autoincrement"`
	TenantID           int64          `bun:"tenant_id,notnull"`
	StudentID          int64          `bun:"student_id,notnull"`
	GuardianProfileID  int64          `bun:"guardian_profile_id,notnull"`
	RelationshipType   string         `bun:"relationship_type,notnull"`
	GuardianRole       string         `bun:"guardian_role,notnull"`
	IsPrimary          bool           `bun:"is_primary,notnull"`
	IsEmergencyContact bool           `bun:"is_emergency_contact,notnull"`
	CanPickup          bool           `bun:"can_pickup,notnull"`
	PickupNotes        *string        `bun:"pickup_notes"`
	EmergencyPriority  int            `bun:"emergency_priority"`
	IsPayer            bool           `bun:"is_payer,notnull"`
	Permissions        map[string]any `bun:"permissions,type:jsonb"`
}

// GuardianStore is the row-level adapter over users.guardian_profiles and
// users.student_guardian_relationships; it shares the database runtime with
// the person store. Link reads that need the pickup permission or the portal
// permissions join the other two owners through the guardian-link projection. The application-level guardian flows stay with the owner's
// legacy service, so this store carries only the reads foreign owners need.
type PortalMembershipQuery func(context.Context, []int64) (map[int64][]int64, error)

type GuardianStore struct {
	database    Database
	memberships PortalMembershipQuery
	links       GuardianLinks
}

// GuardianLinks is People Directory's reader port onto the guardian link
// projection (ADR 0058); the composition root binds it. Each row source is
// aliased "student_guardian" and runs in the caller's statement and
// transaction, with separately named explicit all-school row sources.
type GuardianLinks interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	AllSchoolsGuardianLinkRows(db bun.IDB) *bun.SelectQuery
	GuardianLinkModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
	AllSchoolsGuardianLinkModelRows(db bun.IDB, model any) *bun.SelectQuery
}

// errGuardianLinksUnbound fails a link read closed in a graph that bound no
// guardian link projection.
var errGuardianLinksUnbound = errors.New("people directory postgres: the guardian link projection is required")

func NewGuardianStore(database Database, memberships PortalMembershipQuery, links GuardianLinks) *GuardianStore {
	if database == nil {
		panic("people directory postgres: database runtime is required")
	}
	return &GuardianStore{database: database, memberships: memberships, links: links}
}

// ListLinksByAccounts joins the two owned tables once: every link of every
// profile of the accounts, in the tenant of the context or, inside an admin
// transaction, across every tenant. Profile and link must share the tenant.
func (s *GuardianStore) ListLinksByAccounts(ctx context.Context, accountIDs []int64) ([]peopledirectory.GuardianLink, domain.OperationStats, error) {
	if s.links == nil {
		return nil, domain.OperationStats{}, errGuardianLinksUnbound
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []guardianLinkRow{}
	query := s.linkModelRows(db, scope, &rows).
		Join(`JOIN users.guardian_profiles AS "guardian_profile" ON "guardian_profile".id = "student_guardian".guardian_profile_id AND "guardian_profile".tenant_id = "student_guardian".tenant_id`).
		Where(`"guardian_profile".account_id IN (?)`, bun.List(accountIDs))
	if !scope.AcrossSchools {
		query = query.Where(`"student_guardian".tenant_id = ?`, scope.TenantID)
	}
	query = query.OrderExpr(`"student_guardian".tenant_id ASC, "student_guardian".student_id ASC, "student_guardian".id ASC`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list guardian links by account: %w", err)
	}
	result := make([]peopledirectory.GuardianLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainLink(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *GuardianStore) ListByAccounts(ctx context.Context, accountIDs []int64) ([]domain.Guardian, domain.OperationStats, error) {
	return s.listGuardians(ctx, "list guardians by account", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"guardian_profile".account_id IN (?)`, bun.List(accountIDs))
	})
}

func (s *GuardianStore) ListByIDs(ctx context.Context, ids []int64) ([]domain.Guardian, domain.OperationStats, error) {
	return s.listGuardians(ctx, "list guardians by id", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"guardian_profile".id IN (?)`, bun.List(ids))
	})
}

func (s *GuardianStore) listGuardians(ctx context.Context, operation string, narrow func(*bun.SelectQuery) *bun.SelectQuery) ([]domain.Guardian, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []guardianRow{}
	query := narrow(db.NewSelect().Model(&rows).ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`))
	if !scope.AcrossSchools {
		query = query.Where(`"guardian_profile".tenant_id = ?`, scope.TenantID)
	}
	query = query.OrderExpr(`"guardian_profile".tenant_id ASC, "guardian_profile".id ASC`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: %s: %w", operation, err)
	}
	result := make([]domain.Guardian, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainGuardian(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *GuardianStore) CountLinks(ctx context.Context, guardianIDs []int64) (map[int64]int, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		GuardianProfileID int64 `bun:"guardian_profile_id"`
		Links             int   `bun:"links"`
	}
	query := db.NewSelect().TableExpr(`users.student_guardian_relationships AS "student_guardian"`).
		ColumnExpr(`"student_guardian".guardian_profile_id AS guardian_profile_id`).
		ColumnExpr(`COUNT(*)::int AS links`).
		Where(`"student_guardian".guardian_profile_id IN (?)`, bun.List(guardianIDs))
	if !scope.AcrossSchools {
		query = query.Where(`"student_guardian".tenant_id = ?`, scope.TenantID)
	}
	query = query.GroupExpr(`"student_guardian".guardian_profile_id`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: count guardian links: %w", err)
	}
	result := make(map[int64]int, len(rows))
	for _, row := range rows {
		result[row.GuardianProfileID] = row.Links
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}

// ListAccountLinksByStudents returns the links of the given children whose
// guardian holds a portal account with an active school membership. What such
// a link may do is decided by the caller from its permissions, exactly as for
// ListLinksByAccounts.
func (s *GuardianStore) ListAccountLinksByStudents(ctx context.Context, studentIDs []int64) ([]peopledirectory.GuardianLink, domain.OperationStats, error) {
	if s.memberships == nil {
		return nil, domain.OperationStats{}, fmt.Errorf("people directory postgres: list account links by students: portal account queries are required")
	}
	if s.links == nil {
		return nil, domain.OperationStats{}, errGuardianLinksUnbound
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []struct {
		guardianLinkRow
		AccountID int64 `bun:"portal_account_id"`
	}{}
	query := s.linkModelRows(db, scope, &rows).
		ColumnExpr(`"student_guardian".id, "student_guardian".tenant_id, "student_guardian".student_id,
			"student_guardian".guardian_profile_id, "student_guardian".relationship_type, "student_guardian".guardian_role,
			"student_guardian".is_primary, "student_guardian".is_emergency_contact, "student_guardian".can_pickup,
			"student_guardian".pickup_notes, "student_guardian".emergency_priority, "student_guardian".is_payer,
			"student_guardian".permissions`).
		ColumnExpr(`"guardian_profile".account_id AS portal_account_id`).
		Join(`JOIN users.guardian_profiles AS "guardian_profile" ON "guardian_profile".id = "student_guardian".guardian_profile_id AND "guardian_profile".tenant_id = "student_guardian".tenant_id`).
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"guardian_profile".account_id IS NOT NULL`).
		Where(`"guardian_profile".has_account = true`)
	if !scope.AcrossSchools {
		query = query.Where(`"student_guardian".tenant_id = ?`, scope.TenantID)
	}
	query = query.OrderExpr(`"student_guardian".student_id ASC, "student_guardian".id ASC`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list account links by students: %w", err)
	}
	result := make([]peopledirectory.GuardianLink, 0, len(rows))
	if len(rows) == 0 {
		return result, stats, nil
	}
	accountIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		accountIDs = append(accountIDs, row.AccountID)
	}
	memberships, err := s.memberships(ctx, accountIDs)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: guardian portal memberships: %w", err)
	}
	for _, row := range rows {
		if slices.Contains(memberships[row.AccountID], row.TenantID) {
			result = append(result, toDomainLink(row.guardianLinkRow))
		}
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func toDomainGuardian(row guardianRow) domain.Guardian {
	return domain.Guardian{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		FirstName: row.FirstName, LastName: row.LastName, Email: row.Email,
		AddressStreet: row.AddressStreet, AddressCity: row.AddressCity, AddressPostalCode: row.AddressPostalCode,
		PreferredContactMethod: row.PreferredContactMethod, LanguagePreference: row.LanguagePreference,
		Notes: row.Notes, HasAccount: row.HasAccount, AccountID: row.AccountID,
	}
}

func toDomainLink(row guardianLinkRow) peopledirectory.GuardianLink {
	return peopledirectory.GuardianLink{
		ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID,
		RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole,
		IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup,
		PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer,
		Permissions: domain.GrantedPermissions(row.Permissions),
	}
}

func (s *GuardianStore) linkModelRows(db bun.IDB, scope TenantScope, model any) *bun.SelectQuery {
	if scope.AcrossSchools {
		return s.links.AllSchoolsGuardianLinkModelRows(db, model)
	}
	return s.links.GuardianLinkModelRows(db, scope.TenantID, model)
}
