package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	tenantTagIndex     = "idx_persons_tenant_tag"
	tenantAccountIndex = "idx_persons_tenant_account"
)

// Database resolves the transaction and the tenant of the current request.
// A zero tenant means an admin (cross-tenant) transaction.
type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

type personRow struct {
	bun.BaseModel `bun:"table:persons,alias:person"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	FirstName     string    `bun:"first_name,notnull"`
	LastName      string    `bun:"last_name,notnull"`
	// Birthday is the DATE column as its ISO text. The module cannot use the
	// repository calendar-date type: public contracts may not expose it and
	// the ratchet forbids a new import of it from this owner's internals, so
	// the value travels as YYYY-MM-DD text validated by the public facade.
	Birthday  *string    `bun:"birthday,type:date"`
	TagID     *string    `bun:"tag_id"`
	AccountID *int64     `bun:"account_id"`
	DeletedAt *time.Time `bun:"deleted_at"`
}

func New(database Database) *Store {
	if database == nil {
		panic("people directory postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) Create(ctx context.Context, input domain.CreatePerson) (domain.Person, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Person{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Person{}, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to create a person")
	}
	row := personRow{
		TenantID: tenantID, FirstName: input.FirstName, LastName: input.LastName,
		Birthday: optionalDate(input.Birthday), TagID: input.TagID, AccountID: input.AccountID,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.persons`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Person{}, stats, wrapWriteError("create", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Update(ctx context.Context, input domain.UpdatePerson) (domain.Person, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Person{}, domain.OperationStats{}, err
	}
	row := personRow{
		ID: input.ID, FirstName: input.FirstName, LastName: input.LastName,
		Birthday: optionalDate(input.Birthday), TagID: input.TagID, AccountID: input.AccountID,
	}
	query := db.NewUpdate().Model(&row).
		ModelTableExpr(`users.persons AS "person"`).
		Column("first_name", "last_name", "birthday", "tag_id", "account_id").
		Set(`updated_at = NOW()`).
		Where(`"person".id = ?`, input.ID).
		Where(`"person".deleted_at IS NULL`)
	query = withTenant(query, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Person{}, stats, domain.ErrNotFound
	}
	if err != nil {
		return domain.Person{}, stats, wrapWriteError("update", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) FindByID(ctx context.Context, id int64, lock string) (domain.Person, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Person{}, false, domain.OperationStats{}, err
	}
	row := &personRow{}
	query := withTenant(personSelect(db, row).Where(`"person".id = ?`, id), tenantID)
	if lock != "" {
		query = query.For(lock)
	}
	return scanPerson(ctx, row, query)
}

func (s *Store) FindByAccount(ctx context.Context, accountID int64) (domain.Person, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Person{}, false, domain.OperationStats{}, err
	}
	row := &personRow{}
	query := withTenant(personSelect(db, row).
		Where(`"person".account_id = ?`, accountID).
		Where(`"person".deleted_at IS NULL`).
		OrderExpr(`"person".updated_at DESC, "person".id ASC`).
		Limit(1), tenantID)
	return scanPerson(ctx, row, query)
}

func (s *Store) FindByTag(ctx context.Context, tagID string) (domain.Person, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Person{}, false, domain.OperationStats{}, err
	}
	row := &personRow{}
	query := withTenant(personSelect(db, row).
		Where(`"person".tag_id = ?`, tagID).
		Where(`"person".deleted_at IS NULL`).
		Limit(1), tenantID)
	return scanPerson(ctx, row, query)
}

func scanPerson(ctx context.Context, row *personRow, query *bun.SelectQuery) (domain.Person, bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Person{}, false, stats, nil
	}
	if err != nil {
		return domain.Person{}, false, stats, fmt.Errorf("people directory postgres: find person: %w", err)
	}
	return toDomain(*row), true, stats, nil
}

func (s *Store) ListByIDs(ctx context.Context, ids []int64) ([]domain.Person, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []personRow{}
	query := withTenant(personSelect(db, &rows).
		Where(`"person".id IN (?)`, bun.List(ids)).
		Where(`"person".deleted_at IS NULL`), tenantID)
	return scanPersons(ctx, rows, query.OrderExpr(`"person".id ASC`), "list persons by id")
}

func (s *Store) ListByAccounts(ctx context.Context, accountIDs []int64) ([]domain.Person, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []personRow{}
	query := withTenant(personSelect(db, &rows).
		Where(`"person".account_id IN (?)`, bun.List(accountIDs)).
		Where(`"person".deleted_at IS NULL`), tenantID)
	return scanPersons(ctx, rows, query.OrderExpr(`"person".account_id ASC, "person".updated_at DESC, "person".id ASC`), "list persons by account")
}

func (s *Store) Search(ctx context.Context, filter domain.Filter) ([]domain.Person, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []personRow{}
	query := withTenant(personSelect(db, &rows).Where(`"person".deleted_at IS NULL`), tenantID)
	if filter.FirstNamePrefix != "" {
		query = query.Where(`"person".first_name ILIKE ?`, escapeLike(filter.FirstNamePrefix)+"%")
	}
	if filter.LastNamePrefix != "" {
		query = query.Where(`"person".last_name ILIKE ?`, escapeLike(filter.LastNamePrefix)+"%")
	}
	if filter.FullNameContains != "" {
		query = query.Where(`("person".first_name || ' ' || "person".last_name) ILIKE ?`, "%"+escapeLike(filter.FullNameContains)+"%")
	}
	if filter.TagID != "" {
		query = query.Where(`"person".tag_id = ?`, filter.TagID)
	}
	if len(filter.AccountIDs) > 0 {
		query = query.Where(`"person".account_id IN (?)`, bun.List(filter.AccountIDs))
	}
	query = query.OrderExpr(`"person".last_name ASC, "person".first_name ASC, "person".id ASC`)
	if filter.PageSize > 0 {
		query = query.Limit(filter.PageSize)
		if filter.Page > 1 {
			query = query.Offset((filter.Page - 1) * filter.PageSize)
		}
	}
	return scanPersons(ctx, rows, query, "search persons")
}

func scanPersons(ctx context.Context, rows []personRow, query *bun.SelectQuery, operation string) ([]domain.Person, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: %s: %w", operation, err)
	}
	result := make([]domain.Person, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CountByTenant(ctx context.Context) (map[int64]int, domain.OperationStats, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	type countRow struct {
		TenantID int64 `bun:"tenant_id"`
		Count    int   `bun:"count"`
	}
	var rows []countRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr(`users.persons AS "person"`).
		ColumnExpr(`"person".tenant_id AS tenant_id`).
		ColumnExpr(`COUNT(*) AS count`).
		Where(`"person".deleted_at IS NULL`).
		GroupExpr(`"person".tenant_id`).
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: count persons by tenant: %w", err)
	}
	result := make(map[int64]int, len(rows))
	for _, row := range rows {
		result[row.TenantID] = row.Count
	}
	return result, stats, nil
}

func (s *Store) SoftDelete(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*personRow)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`deleted_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"person".id = ?`, id).
		Where(`"person".deleted_at IS NULL`), tenantID)
	return execOne(ctx, query, "soft delete person")
}

func (s *Store) SetAccount(ctx context.Context, personID int64, accountID *int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*personRow)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`account_id = ?`, accountID).
		Set(`updated_at = NOW()`).
		Where(`"person".id = ?`, personID).
		Where(`"person".deleted_at IS NULL`), tenantID)
	return execOne(ctx, query, "set person account")
}

func (s *Store) SetTag(ctx context.Context, personID int64, tagID *string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*personRow)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`tag_id = ?`, tagID).
		Set(`updated_at = NOW()`).
		Where(`"person".id = ?`, personID).
		Where(`"person".deleted_at IS NULL`), tenantID)
	return execOne(ctx, query, "set person tag")
}

func (s *Store) LockHeldTags(ctx context.Context, personIDs []int64) ([]domain.ReleasedTag, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to release tags")
	}
	type heldRow struct {
		PersonID int64  `bun:"person_id"`
		TagID    string `bun:"tag_id"`
	}
	var rows []heldRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr(`users.persons AS "person"`).
		ColumnExpr(`"person".id AS person_id`).
		ColumnExpr(`"person".tag_id AS tag_id`).
		Where(`"person".id IN (?)`, bun.List(personIDs)).
		Where(`"person".tenant_id = ?`, tenantID).
		Where(`"person".tag_id IS NOT NULL`).
		Where(`"person".deleted_at IS NULL`).
		OrderExpr(`"person".id ASC`).
		For("UPDATE").
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: lock held tags: %w", err)
	}
	result := make([]domain.ReleasedTag, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.ReleasedTag{PersonID: row.PersonID, TagID: row.TagID})
	}
	return result, stats, nil
}

func (s *Store) ClearTags(ctx context.Context, personIDs []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.OperationStats{}, errors.New("people directory postgres: tenant is required to release tags")
	}
	query := db.NewUpdate().Model((*personRow)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`tag_id = NULL`).
		Set(`updated_at = NOW()`).
		Where(`"person".id IN (?)`, bun.List(personIDs)).
		Where(`"person".tenant_id = ?`, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: clear tags: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: count cleared tags: %w", err)
	}
	return stats, nil
}

// RestoreTag re-links tagID to personID when the person holds no tag and no
// other person of the tenant holds tagID. The NOT EXISTS probe reads the
// statement snapshot, so a tag handed out in the gap surfaces as a unique
// violation; the caller runs this inside a savepoint and treats that as
// "current holder wins".
func (s *Store) RestoreTag(ctx context.Context, personID int64, tagID string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return false, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to restore a tag")
	}
	query := db.NewUpdate().Model((*personRow)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`tag_id = ?`, tagID).
		Set(`updated_at = NOW()`).
		Where(`"person".id = ?`, personID).
		Where(`"person".tenant_id = ?`, tenantID).
		Where(`"person".tag_id IS NULL`).
		Where(`"person".deleted_at IS NULL`).
		Where(`NOT EXISTS (SELECT 1 FROM users.persons AS "holder" WHERE "holder".tenant_id = ? AND "holder".tag_id = ?)`, tenantID, tagID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if isUniqueViolationOn(err, tenantTagIndex) {
			return false, stats, domain.ErrTagConflict
		}
		return false, stats, fmt.Errorf("people directory postgres: restore tag: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: count restored tags: %w", err)
	}
	return stats.Rows > 0, stats, nil
}

func execOne(ctx context.Context, query *bun.UpdateQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, wrapWriteError(operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: %s: count rows: %w", operation, err)
	}
	if rows != 1 {
		return stats, domain.ErrNotFound
	}
	stats.Rows = rows
	return stats, nil
}

func personSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.persons AS "person"`)
}

func withTenant[Q interface{ Where(string, ...any) Q }](query Q, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"person".tenant_id = ?`, tenantID)
	}
	return query
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}

func toDomain(row personRow) domain.Person {
	birthday := ""
	if row.Birthday != nil {
		birthday = *row.Birthday
	}
	return domain.Person{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		FirstName: row.FirstName, LastName: row.LastName, Birthday: birthday,
		TagID: row.TagID, AccountID: row.AccountID, DeletedAt: row.DeletedAt,
	}
}

func optionalDate(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func wrapWriteError(operation string, err error) error {
	switch {
	case isUniqueViolationOn(err, tenantTagIndex):
		return domain.ErrTagConflict
	case isUniqueViolationOn(err, tenantAccountIndex):
		return domain.ErrAccountConflict
	}
	return fmt.Errorf("people directory postgres: %s person: %w", operation, err)
}

func isUniqueViolationOn(err error, index string) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == index
}
