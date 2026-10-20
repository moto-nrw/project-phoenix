package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Unique indexes the writes can trip; the names are pinned by the migrations
// that created them. Migration 1.15.409 gave the membership's live-person index
// the name users.staff carried.
const (
	staffPersonIndex       = "idx_staff_tenant_person"
	teacherStaffIndex      = "idx_teachers_tenant_staff"
	teacherStaffConstraint = "teachers_staff_id_key"
	guestStaffIndex        = "idx_guests_tenant_staff"
	guestStaffConstraint   = "guests_staff_id_key"
)

// Database resolves the connection and the tenant of the current request.
// Cross-school administrative access is an explicit scope, never tenant 0.
type Database func(context.Context) (bun.IDB, TenantScope, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("school membership postgres: database runtime is required")
	}
	return &Store{database: database}
}

// staffRow is a staff membership (#2753). The employment half of a staff
// member is Workforce's users.staff_employment_profiles; the application
// service composes it through the StaffEmployment port.
type staffRow struct {
	bun.BaseModel `bun:"table:staff_school_memberships,alias:staff"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	PersonID      int64      `bun:"person_id,notnull"`
	DeletedAt     *time.Time `bun:"deleted_at"`
	// PersonalDataErasedAt marks a tombstone the automatic deletion of
	// staff erased (#3963). The "staff".* reads return it, so the row maps
	// it; scanonly keeps inserts and updates from overwriting the marker.
	PersonalDataErasedAt *time.Time `bun:"personal_data_erased_at,scanonly"`
	// IsGuest and GuestOrganization come from the staff member's guest
	// profile (users.guests, #3823); reads fill them, writes never send them.
	IsGuest           bool   `bun:"is_guest,scanonly"`
	GuestOrganization string `bun:"guest_organization,scanonly"`
}

type teacherRow struct {
	bun.BaseModel  `bun:"table:teachers,alias:teacher"`
	ID             int64      `bun:"id,pk,autoincrement"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID       int64      `bun:"tenant_id,notnull"`
	StaffID        int64      `bun:"staff_id,notnull"`
	Specialization string     `bun:"specialization,nullzero"`
	Role           string     `bun:"role"`
	Qualifications string     `bun:"qualifications"`
	DeletedAt      *time.Time `bun:"deleted_at"`
}

type guestRow struct {
	bun.BaseModel     `bun:"table:guests,alias:guest"`
	ID                int64          `bun:"id,pk,autoincrement"`
	CreatedAt         time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID          int64          `bun:"tenant_id,notnull"`
	StaffID           int64          `bun:"staff_id,notnull"`
	Organization      string         `bun:"organization"`
	ContactEmail      string         `bun:"contact_email"`
	ContactPhone      string         `bun:"contact_phone"`
	ActivityExpertise string         `bun:"activity_expertise,notnull"`
	StartDate         *calendar.Date `bun:"start_date,type:date"`
	EndDate           *calendar.Date `bun:"end_date,type:date"`
	Notes             string         `bun:"notes"`
}

// --- staff ---

func (s *Store) FindStaff(ctx context.Context, id int64, lock string, includeDeleted bool) (schoolmembership.Staff, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Staff{}, false, domain.OperationStats{}, err
	}
	row := &staffRow{}
	query := withTenant(staffSelect(db, row, false).Where(`"staff".id = ?`, id), "staff", scope)
	if !includeDeleted {
		query = query.Where(`"staff".deleted_at IS NULL`)
	}
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find staff")
	if err != nil || !found {
		return schoolmembership.Staff{}, found, stats, err
	}
	return staffToPublic(*row), true, stats, nil
}

func (s *Store) FindStaffByPerson(ctx context.Context, personID int64) (schoolmembership.Staff, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Staff{}, false, domain.OperationStats{}, err
	}
	row := &staffRow{}
	query := withTenant(staffSelect(db, row, false).
		Where(`"staff".person_id = ?`, personID).
		Where(`"staff".deleted_at IS NULL`).
		OrderExpr(`"staff".id ASC`).
		Limit(1), "staff", scope)
	found, stats, err := scanOne(ctx, query, "find staff by person")
	if err != nil || !found {
		return schoolmembership.Staff{}, found, stats, err
	}
	return staffToPublic(*row), true, stats, nil
}

func (s *Store) ListStaff(ctx context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffRow{}
	query := withTenant(staffSelect(db, &rows, filter.IncludeInactiveGuests), "staff", scope)
	if !filter.IncludeDeleted {
		query = query.Where(`"staff".deleted_at IS NULL`)
	}
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []schoolmembership.Staff{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.PersonIDs != nil {
		if len(filter.PersonIDs) == 0 {
			return []schoolmembership.Staff{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff".person_id IN (?)`, bun.List(filter.PersonIDs))
	}
	if filter.TenantIDs != nil {
		if len(filter.TenantIDs) == 0 {
			return []schoolmembership.Staff{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff".tenant_id IN (?)`, bun.List(filter.TenantIDs))
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"staff".id ASC`), "list staff")
	if err != nil {
		return nil, stats, err
	}
	result := make([]schoolmembership.Staff, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffToPublic(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateStaff(ctx context.Context, personID int64) (schoolmembership.Staff, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Staff{}, domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return schoolmembership.Staff{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a staff member")
	}
	row := staffRow{TenantID: scope.TenantID, PersonID: personID}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.staff_school_memberships`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return schoolmembership.Staff{}, stats, wrapStaffWriteError("create", err)
	}
	stats.Rows = 1
	return staffToPublic(row), stats, nil
}

func (s *Store) UpdateStaff(ctx context.Context, id, personID int64) (schoolmembership.Staff, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Staff{}, domain.OperationStats{}, err
	}
	row := staffRow{ID: id, PersonID: personID}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Column("person_id").
		Set(`updated_at = NOW()`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NULL`), "staff", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.Staff{}, stats, schoolmembership.ErrStaffNotFound
	}
	if err != nil {
		return schoolmembership.Staff{}, stats, wrapStaffWriteError("update", err)
	}
	stats.Rows = 1
	return staffToPublic(row), stats, nil
}

func (s *Store) SoftDeleteStaff(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Set(`deleted_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NULL`), "staff", scope)
	return execOne(ctx, query, "soft delete staff", schoolmembership.ErrStaffNotFound)
}

// ReinstateStaff lifts the soft delete of a retired membership (ADR 0021). A
// tombstone the automatic deletion of staff already erased stays retired: the
// condition is re-checked on the locked row, so a concurrent erasure wins.
func (s *Store) ReinstateStaff(ctx context.Context, id int64) (schoolmembership.Staff, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Staff{}, domain.OperationStats{}, err
	}
	row := staffRow{}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Set(`deleted_at = NULL`).
		Set(`updated_at = NOW()`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NOT NULL`).
		Where(`"staff".personal_data_erased_at IS NULL`), "staff", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.Staff{}, stats, schoolmembership.ErrStaffNotFound
	}
	if err != nil {
		return schoolmembership.Staff{}, stats, wrapStaffWriteError("reinstate", err)
	}
	stats.Rows = 1
	return staffToPublic(row), stats, nil
}

// --- teachers ---

func (s *Store) FindTeacher(ctx context.Context, id int64, lock string) (schoolmembership.Teacher, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Teacher{}, false, domain.OperationStats{}, err
	}
	row := &teacherRow{}
	query := withTenant(teacherSelect(db, row).
		Where(`"teacher".id = ?`, id).
		Where(`"teacher".deleted_at IS NULL`), "teacher", scope)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find teacher")
	if err != nil || !found {
		return schoolmembership.Teacher{}, found, stats, err
	}
	return teacherToPublic(*row), true, stats, nil
}

func (s *Store) FindTeacherByStaff(ctx context.Context, staffID int64) (schoolmembership.Teacher, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Teacher{}, false, domain.OperationStats{}, err
	}
	row := &teacherRow{}
	query := withTenant(teacherSelect(db, row).
		Where(`"teacher".staff_id = ?`, staffID).
		Where(`"teacher".deleted_at IS NULL`).
		OrderExpr(`"teacher".id ASC`).
		Limit(1), "teacher", scope)
	found, stats, err := scanOne(ctx, query, "find teacher by staff")
	if err != nil || !found {
		return schoolmembership.Teacher{}, found, stats, err
	}
	return teacherToPublic(*row), true, stats, nil
}

func (s *Store) ListTeachers(ctx context.Context, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []teacherRow{}
	query, ok := teacherListQuery(withTenant(teacherSelect(db, &rows), "teacher", scope), filter)
	if !ok {
		return []schoolmembership.Teacher{}, domain.OperationStats{}, nil
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"teacher".id ASC`), "list teachers")
	if err != nil {
		return nil, stats, err
	}
	result := make([]schoolmembership.Teacher, 0, len(rows))
	for _, row := range rows {
		result = append(result, teacherToPublic(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateTeacher(ctx context.Context, fields schoolmembership.TeacherFields) (schoolmembership.Teacher, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Teacher{}, domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return schoolmembership.Teacher{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a teacher")
	}
	row := teacherRow{
		TenantID: scope.TenantID, StaffID: fields.StaffID, Specialization: fields.Specialization,
		Role: fields.Role, Qualifications: fields.Qualifications,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.teachers`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return schoolmembership.Teacher{}, stats, wrapTeacherWriteError("create", err)
	}
	stats.Rows = 1
	return teacherToPublic(row), stats, nil
}

func (s *Store) UpdateTeacher(ctx context.Context, id int64, fields schoolmembership.TeacherFields) (schoolmembership.Teacher, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Teacher{}, domain.OperationStats{}, err
	}
	row := teacherRow{
		ID: id, StaffID: fields.StaffID, Specialization: fields.Specialization,
		Role: fields.Role, Qualifications: fields.Qualifications,
	}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Column("staff_id", "specialization", "role", "qualifications").
		Set(`updated_at = NOW()`).
		Where(`"teacher".id = ?`, id).
		Where(`"teacher".deleted_at IS NULL`), "teacher", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.Teacher{}, stats, schoolmembership.ErrTeacherNotFound
	}
	if err != nil {
		return schoolmembership.Teacher{}, stats, wrapTeacherWriteError("update", err)
	}
	stats.Rows = 1
	return teacherToPublic(row), stats, nil
}

func (s *Store) SoftDeleteTeacher(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*teacherRow)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set(`deleted_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"teacher".id = ?`, id).
		Where(`"teacher".deleted_at IS NULL`), "teacher", scope)
	return execOne(ctx, query, "soft delete teacher", schoolmembership.ErrTeacherNotFound)
}

// ReinstateTeacher lifts the soft delete of a retired caregiver profile whose
// staff membership is live again (ADR 0021).
func (s *Store) ReinstateTeacher(ctx context.Context, id int64) (schoolmembership.Teacher, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Teacher{}, domain.OperationStats{}, err
	}
	row := teacherRow{}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set(`deleted_at = NULL`).
		Set(`updated_at = NOW()`).
		Where(`"teacher".id = ?`, id).
		Where(`"teacher".deleted_at IS NOT NULL`), "teacher", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.Teacher{}, stats, schoolmembership.ErrTeacherNotFound
	}
	if err != nil {
		return schoolmembership.Teacher{}, stats, wrapTeacherWriteError("reinstate", err)
	}
	stats.Rows = 1
	return teacherToPublic(row), stats, nil
}

// --- guests ---

func (s *Store) FindGuest(ctx context.Context, id int64, lock string) (schoolmembership.Guest, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Guest{}, false, domain.OperationStats{}, err
	}
	row := &guestRow{}
	query := withTenant(guestSelect(db, row).Where(`"guest".id = ?`, id), "guest", scope)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find guest")
	if err != nil || !found {
		return schoolmembership.Guest{}, found, stats, err
	}
	return guestToPublic(*row), true, stats, nil
}

func (s *Store) FindGuestByStaff(ctx context.Context, staffID int64) (schoolmembership.Guest, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Guest{}, false, domain.OperationStats{}, err
	}
	row := &guestRow{}
	query := withTenant(guestSelect(db, row).
		Where(`"guest".staff_id = ?`, staffID).
		OrderExpr(`"guest".id ASC`).
		Limit(1), "guest", scope)
	found, stats, err := scanOne(ctx, query, "find guest by staff")
	if err != nil || !found {
		return schoolmembership.Guest{}, found, stats, err
	}
	return guestToPublic(*row), true, stats, nil
}

func (s *Store) ListGuests(ctx context.Context, filter schoolmembership.GuestFilter) ([]schoolmembership.Guest, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []guestRow{}
	query, ok, err := guestListQuery(withTenant(guestSelect(db, &rows), "guest", scope), filter)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if !ok {
		return []schoolmembership.Guest{}, domain.OperationStats{}, nil
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"guest".id ASC`), "list guests")
	if err != nil {
		return nil, stats, err
	}
	result := make([]schoolmembership.Guest, 0, len(rows))
	for _, row := range rows {
		result = append(result, guestToPublic(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateGuest(ctx context.Context, fields schoolmembership.GuestFields) (schoolmembership.Guest, domain.OperationStats, error) {
	row, err := guestRowFrom(0, fields)
	if err != nil {
		return schoolmembership.Guest{}, domain.OperationStats{}, err
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Guest{}, domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return schoolmembership.Guest{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a guest")
	}
	row.TenantID = scope.TenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.guests`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return schoolmembership.Guest{}, stats, wrapGuestWriteError("create", err)
	}
	stats.Rows = 1
	return guestToPublic(row), stats, nil
}

func (s *Store) UpdateGuest(ctx context.Context, id int64, fields schoolmembership.GuestFields) (schoolmembership.Guest, domain.OperationStats, error) {
	row, err := guestRowFrom(id, fields)
	if err != nil {
		return schoolmembership.Guest{}, domain.OperationStats{}, err
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Guest{}, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.guests AS "guest"`).
		Column("staff_id", "organization", "contact_email", "contact_phone", "activity_expertise", "start_date", "end_date", "notes").
		Set(`updated_at = NOW()`).
		Where(`"guest".id = ?`, id), "guest", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.Guest{}, stats, schoolmembership.ErrGuestNotFound
	}
	if err != nil {
		return schoolmembership.Guest{}, stats, wrapGuestWriteError("update", err)
	}
	stats.Rows = 1
	return guestToPublic(row), stats, nil
}

func (s *Store) DeleteGuest(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*guestRow)(nil)).
		ModelTableExpr(`users.guests AS "guest"`).
		Where(`"guest".id = ?`, id), "guest", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: delete guest: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: delete guest: count rows: %w", err)
	}
	if rows != 1 {
		return stats, schoolmembership.ErrGuestNotFound
	}
	stats.Rows = rows
	return stats, nil
}

// --- helpers ---

// staffSelect reads the membership row together with its guest marker. The
// marker normally follows the guest's activity window; historical readers
// may retain it after that window ended. The guest columns are correlated
// subqueries rather than a join, so FOR UPDATE locks stay on the membership
// row alone.
func staffSelect(db bun.IDB, model any, includeInactiveGuests bool) *bun.SelectQuery {
	query := db.NewSelect().Model(model).ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		ColumnExpr(`"staff".*`)
	if includeInactiveGuests {
		return query.
			ColumnExpr(`EXISTS (SELECT 1 FROM users.guests AS "guest" WHERE "guest".tenant_id = "staff".tenant_id AND "guest".staff_id = "staff".id) AS is_guest`).
			ColumnExpr(`COALESCE((SELECT "guest".organization FROM users.guests AS "guest" WHERE "guest".tenant_id = "staff".tenant_id AND "guest".staff_id = "staff".id), '') AS guest_organization`)
	}
	today := calendar.TodayDate()
	return query.
		ColumnExpr(`EXISTS (SELECT 1 FROM users.guests AS "guest" WHERE "guest".tenant_id = "staff".tenant_id AND "guest".staff_id = "staff".id AND ("guest".start_date IS NULL OR "guest".start_date <= ?) AND ("guest".end_date IS NULL OR "guest".end_date >= ?)) AS is_guest`, today, today).
		ColumnExpr(`COALESCE((SELECT "guest".organization FROM users.guests AS "guest" WHERE "guest".tenant_id = "staff".tenant_id AND "guest".staff_id = "staff".id AND ("guest".start_date IS NULL OR "guest".start_date <= ?) AND ("guest".end_date IS NULL OR "guest".end_date >= ?)), '') AS guest_organization`, today, today)
}

func teacherSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.teachers AS "teacher"`)
}

func guestSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.guests AS "guest"`)
}

func withTenant[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](query Q, alias string, scope TenantScope) Q {
	if err := scope.Validate(); err != nil {
		return query.Err(err)
	}
	if scope.AcrossSchools {
		return query
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, scope.TenantID)
}

func scanOne(ctx context.Context, query *bun.SelectQuery, operation string) (bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return false, stats, nil
	}
	if err != nil {
		return false, stats, fmt.Errorf("school membership postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return true, stats, nil
}

func scanAll(ctx context.Context, query *bun.SelectQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: %s: %w", operation, err)
	}
	return stats, nil
}

func execOne(ctx context.Context, query *bun.UpdateQuery, operation string, notFound error) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: %s: count rows: %w", operation, err)
	}
	if rows != 1 {
		return stats, notFound
	}
	stats.Rows = rows
	return stats, nil
}

func guestRowFrom(id int64, fields schoolmembership.GuestFields) (guestRow, error) {
	start, err := optionalDate(fields.StartDate, "start date")
	if err != nil {
		return guestRow{}, err
	}
	end, err := optionalDate(fields.EndDate, "end date")
	if err != nil {
		return guestRow{}, err
	}
	return guestRow{
		ID: id, StaffID: fields.StaffID, Organization: fields.Organization,
		ContactEmail: fields.ContactEmail, ContactPhone: fields.ContactPhone,
		ActivityExpertise: fields.ActivityExpertise, StartDate: start, EndDate: end, Notes: fields.Notes,
	}, nil
}

func staffToPublic(row staffRow) schoolmembership.Staff {
	return schoolmembership.Staff{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PersonID: row.PersonID, DeletedAt: row.DeletedAt, PersonalDataErasedAt: row.PersonalDataErasedAt,
		IsGuest: row.IsGuest, GuestOrganization: row.GuestOrganization,
	}
}

func teacherToPublic(row teacherRow) schoolmembership.Teacher {
	return schoolmembership.Teacher{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StaffID: row.StaffID, Specialization: row.Specialization, Role: row.Role,
		Qualifications: row.Qualifications, DeletedAt: row.DeletedAt,
	}
}

func guestToPublic(row guestRow) schoolmembership.Guest {
	return schoolmembership.Guest{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StaffID: row.StaffID, Organization: row.Organization, ContactEmail: row.ContactEmail,
		ContactPhone: row.ContactPhone, ActivityExpertise: row.ActivityExpertise,
		StartDate: dateString(row.StartDate), EndDate: dateString(row.EndDate), Notes: row.Notes,
	}
}

func dateString(value *calendar.Date) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func optionalDate(value, label string) (*calendar.Date, error) {
	if value == "" {
		return nil, nil
	}
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, fmt.Errorf("school membership postgres: parse %s: %w", label, err)
	}
	return &date, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}

func wrapStaffWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, staffPersonIndex) {
		return schoolmembership.ErrStaffPersonConflict
	}
	return fmt.Errorf("school membership postgres: %s staff: %w", operation, err)
}

func wrapTeacherWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, teacherStaffIndex, teacherStaffConstraint) {
		return schoolmembership.ErrTeacherStaffConflict
	}
	return fmt.Errorf("school membership postgres: %s teacher: %w", operation, err)
}

func wrapGuestWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, guestStaffIndex, guestStaffConstraint) {
		return schoolmembership.ErrGuestStaffConflict
	}
	return fmt.Errorf("school membership postgres: %s guest: %w", operation, err)
}

func isUniqueViolationOn(err error, names ...string) bool {
	var postgresError pgdriver.Error
	if !errors.As(err, &postgresError) || !postgresError.IntegrityViolation() {
		return false
	}
	constraint := postgresError.Field('n')
	for _, name := range names {
		if constraint == name {
			return true
		}
	}
	return false
}
