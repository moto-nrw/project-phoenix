package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Unique indexes the writes can trip; the names are pinned by the migrations
// that created them.
const (
	staffPersonIndex          = "idx_staff_tenant_person"
	staffPersonConstraint     = "staff_person_id_key"
	staffPersonnelNumberIndex = "uq_staff_tenant_personnel_number"
	teacherStaffIndex         = "idx_teachers_tenant_staff"
	teacherStaffConstraint    = "teachers_staff_id_key"
	guestStaffIndex           = "idx_guests_tenant_staff"
	guestStaffConstraint      = "guests_staff_id_key"
)

// Database resolves the connection and the tenant of the current request.
// A zero tenant means an admin (cross-tenant) transaction.
type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("school membership postgres: database runtime is required")
	}
	return &Store{database: database}
}

type staffRow struct {
	bun.BaseModel         `bun:"table:staff,alias:staff"`
	ID                    int64          `bun:"id,pk,autoincrement"`
	CreatedAt             time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID              int64          `bun:"tenant_id,notnull"`
	PersonID              int64          `bun:"person_id,notnull"`
	StaffNotes            string         `bun:"staff_notes"`
	EmploymentType        *string        `bun:"employment_type"`
	WorkTimeModelID       *int64         `bun:"work_time_model_id"`
	PersonnelNumber       *string        `bun:"personnel_number"`
	RotationAnchorDate    *calendar.Date `bun:"rotation_anchor_date,type:date"`
	BirthdayDisplayOptOut bool           `bun:"birthday_display_opt_out,notnull"`
	DeletedAt             *time.Time     `bun:"deleted_at"`
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

func (s *Store) FindStaff(ctx context.Context, id int64, lock string, includeDeleted bool) (domain.Staff, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Staff{}, false, domain.OperationStats{}, err
	}
	row := &staffRow{}
	query := withTenant(staffSelect(db, row).Where(`"staff".id = ?`, id), "staff", tenantID)
	if !includeDeleted {
		query = query.Where(`"staff".deleted_at IS NULL`)
	}
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find staff")
	if err != nil || !found {
		return domain.Staff{}, found, stats, err
	}
	return staffToDomain(*row), true, stats, nil
}

func (s *Store) FindStaffByPerson(ctx context.Context, personID int64) (domain.Staff, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Staff{}, false, domain.OperationStats{}, err
	}
	row := &staffRow{}
	query := withTenant(staffSelect(db, row).
		Where(`"staff".person_id = ?`, personID).
		Where(`"staff".deleted_at IS NULL`).
		OrderExpr(`"staff".id ASC`).
		Limit(1), "staff", tenantID)
	found, stats, err := scanOne(ctx, query, "find staff by person")
	if err != nil || !found {
		return domain.Staff{}, found, stats, err
	}
	return staffToDomain(*row), true, stats, nil
}

func (s *Store) ListStaff(ctx context.Context, filter domain.StaffFilter) ([]domain.Staff, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffRow{}
	query := withTenant(staffSelect(db, &rows), "staff", tenantID)
	if !filter.IncludeDeleted {
		query = query.Where(`"staff".deleted_at IS NULL`)
	}
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.Staff{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.PersonIDs != nil {
		if len(filter.PersonIDs) == 0 {
			return []domain.Staff{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff".person_id IN (?)`, bun.List(filter.PersonIDs))
	}
	if filter.WorkTimeModelID != nil {
		query = query.Where(`"staff".work_time_model_id = ?`, *filter.WorkTimeModelID)
	}
	if filter.TenantIDs != nil {
		if len(filter.TenantIDs) == 0 {
			return []domain.Staff{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff".tenant_id IN (?)`, bun.List(filter.TenantIDs))
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"staff".id ASC`), "list staff")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Staff, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateStaff(ctx context.Context, fields domain.StaffFields) (domain.Staff, domain.OperationStats, error) {
	anchor, err := optionalDate(fields.RotationAnchorDate, "rotation anchor date")
	if err != nil {
		return domain.Staff{}, domain.OperationStats{}, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Staff{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Staff{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a staff member")
	}
	row := staffRow{
		TenantID: tenantID, PersonID: fields.PersonID, StaffNotes: fields.StaffNotes,
		EmploymentType: fields.EmploymentType, WorkTimeModelID: fields.WorkTimeModelID,
		PersonnelNumber: fields.PersonnelNumber, RotationAnchorDate: anchor,
		BirthdayDisplayOptOut: fields.BirthdayDisplayOptOut,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.staff`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Staff{}, stats, wrapStaffWriteError("create", err)
	}
	stats.Rows = 1
	return staffToDomain(row), stats, nil
}

func (s *Store) UpdateStaff(ctx context.Context, id int64, fields domain.StaffFields) (domain.Staff, domain.OperationStats, error) {
	anchor, err := optionalDate(fields.RotationAnchorDate, "rotation anchor date")
	if err != nil {
		return domain.Staff{}, domain.OperationStats{}, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Staff{}, domain.OperationStats{}, err
	}
	row := staffRow{
		ID: id, PersonID: fields.PersonID, StaffNotes: fields.StaffNotes,
		EmploymentType: fields.EmploymentType, WorkTimeModelID: fields.WorkTimeModelID,
		PersonnelNumber: fields.PersonnelNumber, RotationAnchorDate: anchor,
		BirthdayDisplayOptOut: fields.BirthdayDisplayOptOut,
	}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.staff AS "staff"`).
		Column("person_id", "staff_notes", "employment_type", "work_time_model_id", "personnel_number", "rotation_anchor_date", "birthday_display_opt_out").
		Set(`updated_at = NOW()`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NULL`), "staff", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Staff{}, stats, domain.ErrStaffNotFound
	}
	if err != nil {
		return domain.Staff{}, stats, wrapStaffWriteError("update", err)
	}
	stats.Rows = 1
	return staffToDomain(row), stats, nil
}

func (s *Store) SoftDeleteStaff(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`deleted_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NULL`), "staff", tenantID)
	return execOne(ctx, query, "soft delete staff", domain.ErrStaffNotFound)
}

func (s *Store) ClearWorkTimeModel(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`work_time_model_id = NULL`).
		Where(`"staff".id = ?`, id), "staff", tenantID)
	return execOne(ctx, query, "clear work time model", domain.ErrStaffNotFound)
}

func (s *Store) SetStaffNotes(ctx context.Context, id int64, notes string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`staff_notes = ?`, notes).
		Set(`updated_at = NOW()`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NULL`), "staff", tenantID)
	return execOne(ctx, query, "set staff notes", domain.ErrStaffNotFound)
}

func (s *Store) SetBirthdayDisplayOptOut(ctx context.Context, id int64, optOut bool) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`birthday_display_opt_out = ?`, optOut).
		Set(`updated_at = CURRENT_TIMESTAMP`).
		Where(`"staff".id = ?`, id).
		Where(`"staff".deleted_at IS NULL`), "staff", tenantID)
	return execOne(ctx, query, "set staff birthday display opt-out", domain.ErrStaffNotFound)
}

// RebaseWorkTimeModelAnchor stamps the anchor onto every live staff member
// of the template and returns their IDs in ascending order.
func (s *Store) RebaseWorkTimeModelAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, domain.OperationStats, error) {
	anchor, err := optionalDate(anchorDate, "rotation anchor date")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`rotation_anchor_date = ?`, anchor).
		Where(`"staff".work_time_model_id = ?`, workTimeModelID).
		Where(`"staff".deleted_at IS NULL`), "staff", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning(`"staff".id`).Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, stats, fmt.Errorf("school membership postgres: rebase work time model anchor: %w", err)
	}
	stats.Rows = int64(len(ids))
	sortInt64(ids)
	return ids, stats, nil
}

// --- teachers ---

func (s *Store) FindTeacher(ctx context.Context, id int64, lock string) (domain.Teacher, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Teacher{}, false, domain.OperationStats{}, err
	}
	row := &teacherRow{}
	query := withTenant(teacherSelect(db, row).
		Where(`"teacher".id = ?`, id).
		Where(`"teacher".deleted_at IS NULL`), "teacher", tenantID)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find teacher")
	if err != nil || !found {
		return domain.Teacher{}, found, stats, err
	}
	return teacherToDomain(*row), true, stats, nil
}

func (s *Store) FindTeacherByStaff(ctx context.Context, staffID int64) (domain.Teacher, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Teacher{}, false, domain.OperationStats{}, err
	}
	row := &teacherRow{}
	query := withTenant(teacherSelect(db, row).
		Where(`"teacher".staff_id = ?`, staffID).
		Where(`"teacher".deleted_at IS NULL`).
		OrderExpr(`"teacher".id ASC`).
		Limit(1), "teacher", tenantID)
	found, stats, err := scanOne(ctx, query, "find teacher by staff")
	if err != nil || !found {
		return domain.Teacher{}, found, stats, err
	}
	return teacherToDomain(*row), true, stats, nil
}

func (s *Store) ListTeachers(ctx context.Context, filter domain.TeacherFilter) ([]domain.Teacher, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []teacherRow{}
	query := withTenant(teacherSelect(db, &rows), "teacher", tenantID)
	if !filter.IncludeDeleted {
		query = query.Where(`"teacher".deleted_at IS NULL`)
	}
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.Teacher{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"teacher".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			return []domain.Teacher{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"teacher".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.Specialization != "" {
		query = query.Where(`LOWER("teacher".specialization) = LOWER(?)`, filter.Specialization)
	}
	if filter.SpecializationContains != "" {
		query = query.Where(`"teacher".specialization ILIKE ?`, "%"+escapeLike(filter.SpecializationContains)+"%")
	}
	if filter.RoleContains != "" {
		query = query.Where(`"teacher".role ILIKE ?`, "%"+escapeLike(filter.RoleContains)+"%")
	}
	if filter.HasQualifications != nil {
		if *filter.HasQualifications {
			query = query.Where(`"teacher".qualifications IS NOT NULL AND "teacher".qualifications <> ''`)
		} else {
			query = query.Where(`("teacher".qualifications IS NULL OR "teacher".qualifications = '')`)
		}
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"teacher".id ASC`), "list teachers")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Teacher, 0, len(rows))
	for _, row := range rows {
		result = append(result, teacherToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateTeacher(ctx context.Context, fields domain.TeacherFields) (domain.Teacher, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Teacher{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Teacher{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a teacher")
	}
	row := teacherRow{
		TenantID: tenantID, StaffID: fields.StaffID, Specialization: fields.Specialization,
		Role: fields.Role, Qualifications: fields.Qualifications,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.teachers`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Teacher{}, stats, wrapTeacherWriteError("create", err)
	}
	stats.Rows = 1
	return teacherToDomain(row), stats, nil
}

func (s *Store) UpdateTeacher(ctx context.Context, id int64, fields domain.TeacherFields) (domain.Teacher, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Teacher{}, domain.OperationStats{}, err
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
		Where(`"teacher".deleted_at IS NULL`), "teacher", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Teacher{}, stats, domain.ErrTeacherNotFound
	}
	if err != nil {
		return domain.Teacher{}, stats, wrapTeacherWriteError("update", err)
	}
	stats.Rows = 1
	return teacherToDomain(row), stats, nil
}

func (s *Store) SoftDeleteTeacher(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*teacherRow)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set(`deleted_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"teacher".id = ?`, id).
		Where(`"teacher".deleted_at IS NULL`), "teacher", tenantID)
	return execOne(ctx, query, "soft delete teacher", domain.ErrTeacherNotFound)
}

// --- guests ---

func (s *Store) FindGuest(ctx context.Context, id int64, lock string) (domain.Guest, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Guest{}, false, domain.OperationStats{}, err
	}
	row := &guestRow{}
	query := withTenant(guestSelect(db, row).Where(`"guest".id = ?`, id), "guest", tenantID)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find guest")
	if err != nil || !found {
		return domain.Guest{}, found, stats, err
	}
	return guestToDomain(*row), true, stats, nil
}

func (s *Store) FindGuestByStaff(ctx context.Context, staffID int64) (domain.Guest, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Guest{}, false, domain.OperationStats{}, err
	}
	row := &guestRow{}
	query := withTenant(guestSelect(db, row).
		Where(`"guest".staff_id = ?`, staffID).
		OrderExpr(`"guest".id ASC`).
		Limit(1), "guest", tenantID)
	found, stats, err := scanOne(ctx, query, "find guest by staff")
	if err != nil || !found {
		return domain.Guest{}, found, stats, err
	}
	return guestToDomain(*row), true, stats, nil
}

func (s *Store) ListGuests(ctx context.Context, filter domain.GuestFilter) ([]domain.Guest, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []guestRow{}
	query := withTenant(guestSelect(db, &rows), "guest", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.Guest{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"guest".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			return []domain.Guest{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"guest".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.ActiveOn != "" {
		activeOn, err := optionalDate(filter.ActiveOn, "active-on date")
		if err != nil {
			return nil, domain.OperationStats{}, err
		}
		query = query.Where(`("guest".start_date IS NULL OR "guest".start_date <= ?)`, activeOn).
			Where(`("guest".end_date IS NULL OR "guest".end_date >= ?)`, activeOn)
	}
	if filter.OrganizationContains != "" {
		query = query.Where(`"guest".organization ILIKE ?`, "%"+escapeLike(filter.OrganizationContains)+"%")
	}
	if filter.ExpertiseContains != "" {
		query = query.Where(`"guest".activity_expertise ILIKE ?`, "%"+escapeLike(filter.ExpertiseContains)+"%")
	}
	if filter.HasOrganization != nil {
		if *filter.HasOrganization {
			query = query.Where(`"guest".organization IS NOT NULL`)
		} else {
			query = query.Where(`"guest".organization IS NULL`)
		}
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"guest".id ASC`), "list guests")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Guest, 0, len(rows))
	for _, row := range rows {
		result = append(result, guestToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateGuest(ctx context.Context, fields domain.GuestFields) (domain.Guest, domain.OperationStats, error) {
	row, err := guestRowFrom(0, fields)
	if err != nil {
		return domain.Guest{}, domain.OperationStats{}, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Guest{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.Guest{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a guest")
	}
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`users.guests`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Guest{}, stats, wrapGuestWriteError("create", err)
	}
	stats.Rows = 1
	return guestToDomain(row), stats, nil
}

func (s *Store) UpdateGuest(ctx context.Context, id int64, fields domain.GuestFields) (domain.Guest, domain.OperationStats, error) {
	row, err := guestRowFrom(id, fields)
	if err != nil {
		return domain.Guest{}, domain.OperationStats{}, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Guest{}, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`users.guests AS "guest"`).
		Column("staff_id", "organization", "contact_email", "contact_phone", "activity_expertise", "start_date", "end_date", "notes").
		Set(`updated_at = NOW()`).
		Where(`"guest".id = ?`, id), "guest", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Guest{}, stats, domain.ErrGuestNotFound
	}
	if err != nil {
		return domain.Guest{}, stats, wrapGuestWriteError("update", err)
	}
	stats.Rows = 1
	return guestToDomain(row), stats, nil
}

func (s *Store) DeleteGuest(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*guestRow)(nil)).
		ModelTableExpr(`users.guests AS "guest"`).
		Where(`"guest".id = ?`, id), "guest", tenantID)
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
		return stats, domain.ErrGuestNotFound
	}
	stats.Rows = rows
	return stats, nil
}

// --- helpers ---

func staffSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.staff AS "staff"`)
}

func teacherSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.teachers AS "teacher"`)
}

func guestSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.guests AS "guest"`)
}

func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
	}
	return query
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

func guestRowFrom(id int64, fields domain.GuestFields) (guestRow, error) {
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

func staffToDomain(row staffRow) domain.Staff {
	return domain.Staff{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PersonID: row.PersonID, StaffNotes: row.StaffNotes, EmploymentType: row.EmploymentType,
		WorkTimeModelID: row.WorkTimeModelID, PersonnelNumber: row.PersonnelNumber,
		RotationAnchorDate: dateString(row.RotationAnchorDate), BirthdayDisplayOptOut: row.BirthdayDisplayOptOut,
		DeletedAt: row.DeletedAt,
	}
}

func teacherToDomain(row teacherRow) domain.Teacher {
	return domain.Teacher{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StaffID: row.StaffID, Specialization: row.Specialization, Role: row.Role,
		Qualifications: row.Qualifications, DeletedAt: row.DeletedAt,
	}
}

func guestToDomain(row guestRow) domain.Guest {
	return domain.Guest{
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

func sortInt64(values []int64) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j-1] > values[j]; j-- {
			values[j-1], values[j] = values[j], values[j-1]
		}
	}
}

func wrapStaffWriteError(operation string, err error) error {
	switch {
	case isUniqueViolationOn(err, staffPersonIndex, staffPersonConstraint):
		return domain.ErrStaffPersonConflict
	case isUniqueViolationOn(err, staffPersonnelNumberIndex):
		return domain.ErrPersonnelNumberConflict
	}
	return fmt.Errorf("school membership postgres: %s staff: %w", operation, err)
}

func wrapTeacherWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, teacherStaffIndex, teacherStaffConstraint) {
		return domain.ErrTeacherStaffConflict
	}
	return fmt.Errorf("school membership postgres: %s teacher: %w", operation, err)
}

func wrapGuestWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, guestStaffIndex, guestStaffConstraint) {
		return domain.ErrGuestStaffConflict
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
