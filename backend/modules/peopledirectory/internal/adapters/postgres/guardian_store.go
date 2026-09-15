package postgres

import (
	"context"
	"fmt"
	"time"

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
	bun.BaseModel      `bun:"table:students_guardians,alias:student_guardian"`
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
// users.students_guardians; it shares the database runtime with the person
// store. The application-level guardian flows stay with the owner's
// legacy service, so this store carries only the reads foreign owners need.
type GuardianStore struct{ database Database }

func NewGuardianStore(database Database) *GuardianStore {
	if database == nil {
		panic("people directory postgres: database runtime is required")
	}
	return &GuardianStore{database: database}
}

// ListLinksByAccount joins the two owned tables once: every link of every
// profile of the account, in the tenant of the context or, inside an admin
// transaction, across every tenant. Profile and link must share the tenant.
func (s *GuardianStore) ListLinksByAccount(ctx context.Context, accountID int64) ([]domain.GuardianLink, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []guardianLinkRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Join(`JOIN users.guardian_profiles AS "guardian_profile" ON "guardian_profile".id = "student_guardian".guardian_profile_id AND "guardian_profile".tenant_id = "student_guardian".tenant_id`).
		Where(`"guardian_profile".account_id = ?`, accountID)
	if tenantID > 0 {
		query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
	}
	query = query.OrderExpr(`"student_guardian".tenant_id ASC, "student_guardian".student_id ASC, "student_guardian".id ASC`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list guardian links by account: %w", err)
	}
	result := make([]domain.GuardianLink, 0, len(rows))
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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []guardianRow{}
	query := narrow(db.NewSelect().Model(&rows).ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`))
	if tenantID > 0 {
		query = query.Where(`"guardian_profile".tenant_id = ?`, tenantID)
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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		GuardianProfileID int64 `bun:"guardian_profile_id"`
		Links             int   `bun:"links"`
	}
	query := db.NewSelect().TableExpr(`users.students_guardians AS "student_guardian"`).
		ColumnExpr(`"student_guardian".guardian_profile_id AS guardian_profile_id`).
		ColumnExpr(`COUNT(*)::int AS links`).
		Where(`"student_guardian".guardian_profile_id IN (?)`, bun.List(guardianIDs))
	if tenantID > 0 {
		query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
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

func toDomainGuardian(row guardianRow) domain.Guardian {
	return domain.Guardian{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		FirstName: row.FirstName, LastName: row.LastName, Email: row.Email,
		AddressStreet: row.AddressStreet, AddressCity: row.AddressCity, AddressPostalCode: row.AddressPostalCode,
		PreferredContactMethod: row.PreferredContactMethod, LanguagePreference: row.LanguagePreference,
		Notes: row.Notes, HasAccount: row.HasAccount, AccountID: row.AccountID,
	}
}

func toDomainLink(row guardianLinkRow) domain.GuardianLink {
	return domain.GuardianLink{
		ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID,
		RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole,
		IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup,
		PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer,
		Permissions: domain.GrantedPermissions(row.Permissions),
	}
}
