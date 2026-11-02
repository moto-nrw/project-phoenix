package postgres

import (
	"context"
	"fmt"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// directoryRecordRow is the whole users.students row. The departure plans are
// selected in the same statement as everything else: the retained repository
// needed a second query for them because its generic list could not reach
// scan-only columns, and one statement is both cheaper and atomic.
type directoryRecordRow struct {
	ID        int64     `bun:"id"`
	CreatedAt time.Time `bun:"created_at"`
	UpdatedAt time.Time `bun:"updated_at"`
	TenantID  int64     `bun:"tenant_id"`

	PersonID    int64  `bun:"person_id"`
	SchoolClass string `bun:"school_class"`
	GroupID     *int64 `bun:"group_id"`
	Status      string `bun:"status"`

	EnrolledFrom  *calendar.Date `bun:"enrolled_from"`
	EnrolledUntil *calendar.Date `bun:"enrolled_until"`

	AddressStreet     *string `bun:"address_street"`
	AddressCity       *string `bun:"address_city"`
	AddressPostalCode *string `bun:"address_postal_code"`

	ExtraInfo    *string `bun:"extra_info"`
	HealthInfo   *string `bun:"health_info"`
	PickupStatus *string `bun:"pickup_status"`

	DepartureDays          directory.DepartureDays         `bun:"departure_days"`
	AllowedDepartureModes  directory.AllowedDepartureModes `bun:"allowed_departure_modes"`
	PickupDays             directory.PickupDays            `bun:"pickup_days"`
	BusDays                directory.BusDays               `bun:"bus_days"`
	DepartureCompanionNote *string                         `bun:"departure_companion_note"`

	Sick         *bool      `bun:"sick"`
	SickSince    *time.Time `bun:"sick_since"`
	Excused      *bool      `bun:"excused"`
	ExcusedSince *time.Time `bun:"excused_since"`

	PhotoPath           *string    `bun:"photo_path"`
	PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at"`
	PhotoConsentGivenBy *int64     `bun:"photo_consent_given_by"`

	AGBAcceptedAt            *time.Time `bun:"agb_accepted_at"`
	DataProcessingAcceptedAt *time.Time `bun:"data_processing_accepted_at"`
	EmailContactAcceptedAt   *time.Time `bun:"email_contact_accepted_at"`
}

func (r directoryRecordRow) toDomain() directory.StudentRecord {
	record := directory.StudentRecord{
		ID: r.ID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, TenantID: r.TenantID,
		PersonID: r.PersonID, SchoolClass: r.SchoolClass, GroupID: r.GroupID, Status: r.Status,

		AddressStreet: r.AddressStreet, AddressCity: r.AddressCity, AddressPostalCode: r.AddressPostalCode,
		ExtraInfo:  r.ExtraInfo,
		HealthInfo: r.HealthInfo, PickupStatus: r.PickupStatus,
		DepartureDays: r.DepartureDays, AllowedDepartureModes: r.AllowedDepartureModes,
		PickupDays: r.PickupDays, BusDays: r.BusDays, DepartureCompanionNote: r.DepartureCompanionNote,
		Sick: r.Sick, SickSince: r.SickSince, Excused: r.Excused, ExcusedSince: r.ExcusedSince,
		PhotoPath: r.PhotoPath, PhotoConsentGivenAt: r.PhotoConsentGivenAt, PhotoConsentGivenBy: r.PhotoConsentGivenBy,
		AGBAcceptedAt: r.AGBAcceptedAt, DataProcessingAcceptedAt: r.DataProcessingAcceptedAt,
		EmailContactAcceptedAt: r.EmailContactAcceptedAt,
	}
	if r.EnrolledFrom != nil {
		record.EnrolledFrom = r.EnrolledFrom.String()
	}
	if r.EnrolledUntil != nil {
		record.EnrolledUntil = r.EnrolledUntil.String()
	}
	return record
}

const studentRecordColumns = `"student".id, "student".created_at, "student".updated_at, "student".tenant_id,
	"student".person_id, "student".school_class, "student".group_id, "student".status,
	"student".enrolled_from, "student".enrolled_until,
	"student".address_street, "student".address_city, "student".address_postal_code,
	"student".extra_info, "student".health_info, "student".pickup_status,
	"student".departure_days, "student".allowed_departure_modes, "student".pickup_days, "student".bus_days,
	"student".departure_companion_note,
	"student".sick, "student".sick_since, "student".excused, "student".excused_since,
	"student".photo_path, "student".photo_consent_given_at, "student".photo_consent_given_by,
	"student".agb_accepted_at, "student".data_processing_accepted_at, "student".email_contact_accepted_at`

func (s *studentDirectoryReads) ListDirectory(
	ctx context.Context,
	filter directoryFilter,
) ([]directory.StudentRecord, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryRecordRow{}
	query := applyStudentDirectoryFilter(
		withStudentTenant(s.studentRows(db, scope).
			ColumnExpr(studentRecordColumns), scope),
		filter,
	)
	// Without a total order PostgreSQL may hand the same child back on two
	// pages of one selection and never mention another, so the primary key is
	// always the last word on ordering.
	query = query.OrderExpr(`"student".id ASC`)
	if filter.PageSize > 0 {
		page := max(filter.Page, 1)
		query = query.Limit(int64(filter.PageSize)).Offset(int64((page - 1) * filter.PageSize))
	}

	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list student directory: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]directory.StudentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *studentDirectoryReads) CountDirectory(
	ctx context.Context,
	filter directoryFilter,
) (int, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, directoryStats{}, err
	}
	query := applyStudentDirectoryFilter(
		withStudentTenant(s.studentRows(db, scope), scope),
		filter,
	)
	stats := directoryStats{Queries: 1}
	started := time.Now()
	total, err := query.Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("student directory projection: count student directory: %w", err)
	}
	stats.Rows = int64(total)
	return int(total), stats, nil
}

func (s *studentDirectoryReads) ListDirectoryIDs(ctx context.Context) ([]int64, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	ids := []int64{}
	query := withStudentTenant(s.studentRows(db, scope), scope).
		ColumnExpr(`"student".id`).
		Where(`"student".status != ?`, directory.StudentStatusAlumnus).
		OrderExpr(`"student".id ASC`)
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list student directory ids: %w", err)
	}
	stats.Rows = int64(len(ids))
	return ids, stats, nil
}

// applyStudentDirectoryFilter reproduces the retained generic filter tree
// exactly: the same trimmed/lower-cased class comparison, the same
// first-number grade match, the same guardian substring, and the same two
// visibility groups. A graduate is out unless the caller names it, and the
// care window is the half-open enrolment interval around the caller's day.
func applyStudentDirectoryFilter(query *bun.SelectQuery, filter directoryFilter) *bun.SelectQuery {
	query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
		group = group.WhereOr(`"student".status != ?`, directory.StudentStatusAlumnus)
		if len(filter.KeepAlumni) > 0 {
			group = group.WhereOr(`"student".id IN (?)`, bun.List(filter.KeepAlumni))
		}
		return group
	})

	switch filter.CareStatus {
	case directory.StudentCareStatusEnded:
		query = query.Where(`"student".enrolled_until IS NOT NULL`).
			Where(`"student".enrolled_until < ?`, filter.CareStatusOn)
	case directory.StudentCareStatusAll:
		// No boundary: the caller manages both sides.
	default:
		query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			return group.WhereOr(`"student".enrolled_until IS NULL`).
				WhereOr(`"student".enrolled_until >= ?`, filter.CareStatusOn)
		})
	}

	if len(filter.SchoolClasses) > 0 {
		classes := filter.SchoolClasses
		query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			for _, class := range classes {
				group = group.WhereOr(`LOWER(TRIM("student".school_class)) = LOWER(TRIM(?))`, class)
			}
			return group
		})
	}
	if len(filter.GradeLevels) > 0 {
		query = query.Where(`substring("student".school_class from '[0-9]+') IN (?)`, bun.List(filter.GradeLevels))
	}
	if len(filter.IDs) > 0 {
		query = query.Where(`"student".id IN (?)`, bun.List(filter.IDs))
	}
	return query
}

// FindRecord reads one owned row, optionally under a row lock the caller's
// transaction holds until it commits.
func (s *studentDirectoryReads) FindRecord(
	ctx context.Context,
	studentID int64,
	lock string,
) (directory.StudentRecord, bool, directoryStats, error) {
	return s.findRecord(ctx, studentID, lock, nil)
}

// findRecord reads one owned row. A locking read given a gate query joins
// the gate as a materialized CTE, so one statement takes the gate before the
// row lock.
func (s *studentDirectoryReads) findRecord(
	ctx context.Context,
	studentID int64,
	lock string,
	gate *bun.SelectQuery,
) (directory.StudentRecord, bool, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return directory.StudentRecord{}, false, directoryStats{}, err
	}
	rows := []directoryRecordRow{}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(studentRecordColumns).Where(`"student".id = ?`, studentID), scope)
	scan := query.Scan
	switch {
	case lock != "" && gate != nil:
		scan = db.NewRaw(`WITH class_write_gate AS MATERIALIZED (?)
  SELECT record.* FROM (?) AS record CROSS JOIN class_write_gate
  FOR ?`, gate, query, bun.Safe(lock)).Scan
	case lock != "":
		scan = query.For(lock).Scan
	}
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		// A NOWAIT acquisition the holder refuses is the caller's retriable
		// conflict, not a failure of this read.
		if isLockNotAvailable(err) {
			return directory.StudentRecord{}, false, stats,
				fmt.Errorf("student directory projection: find student record: %w", directory.ErrStudentLockBusy)
		}
		return directory.StudentRecord{}, false, stats, fmt.Errorf("student directory projection: find student record: %w", err)
	}
	if len(rows) == 0 {
		return directory.StudentRecord{}, false, stats, nil
	}
	stats.Rows = 1
	return rows[0].toDomain(), true, stats, nil
}

// ListRecordsByIDs reads the owned rows of the given children, alumni
// included, ordered by id.
func (s *studentDirectoryReads) ListRecordsByIDs(
	ctx context.Context,
	ids []int64,
) ([]directory.StudentRecord, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryRecordRow{}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(studentRecordColumns).Where(`"student".id IN (?)`, bun.List(ids)), scope).
		OrderExpr(`"student".id ASC`)
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list student records: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]directory.StudentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}
