package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The Elternsprechtag days and bookings (#4801). Every operation needs the
// tenant and joins the caller's transaction; a write validates its values
// before it reaches the table.

const (
	parentConferenceTable = `calendar.parent_conferences AS "parent_conference"`
	parentBookingTable    = `calendar.parent_conference_bookings AS "parent_booking"`

	parentBookingSlotConstraint    = "uq_calendar_parent_conference_bookings_slot"
	parentBookingStudentConstraint = "uq_calendar_parent_conference_bookings_student"
)

type parentConferenceRow struct {
	bun.BaseModel      `bun:"table:calendar.parent_conferences,alias:parent_conference"`
	ID                 int64                                  `bun:"id,pk,autoincrement"`
	TenantID           int64                                  `bun:"tenant_id,notnull"`
	CreatedAt          time.Time                              `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time                              `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	OrganizerStaffID   int64                                  `bun:"organizer_staff_id,notnull"`
	OrganizerAccountID int64                                  `bun:"organizer_account_id,nullzero"`
	SchoolClass        string                                 `bun:"school_class,notnull"`
	ConferenceDate     calendar.Date                          `bun:"conference_date,notnull,type:date"`
	StartMinute        int                                    `bun:"start_minute,notnull"`
	EndMinute          int                                    `bun:"end_minute,notnull"`
	SlotMinutes        int                                    `bun:"slot_minutes,notnull"`
	Breaks             []schoolcalendar.ParentConferenceBreak `bun:"breaks,type:jsonb,notnull"`
	Location           *string                                `bun:"location"`
}

type parentBookingRow struct {
	bun.BaseModel     `bun:"table:calendar.parent_conference_bookings,alias:parent_booking"`
	ID                int64     `bun:"id,pk,autoincrement"`
	TenantID          int64     `bun:"tenant_id,notnull"`
	CreatedAt         time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	ConferenceID      int64     `bun:"conference_id,notnull"`
	StudentID         int64     `bun:"student_id,notnull"`
	GuardianProfileID *int64    `bun:"guardian_profile_id"`
	SlotStartMinute   int       `bun:"slot_start_minute,notnull"`
	AppointmentID     *int64    `bun:"appointment_id"`
}

// ParentConferences is the store of the days and bookings.
type ParentConferences struct{ *Store }

var _ schoolcalendar.ParentConferences = ParentConferences{}

func (s ParentConferences) CreateParentConference(ctx context.Context, fields schoolcalendar.ParentConferenceFields) (*schoolcalendar.ParentConference, error) {
	fields, err := schoolcalendar.ValidateParentConferenceFields(fields)
	if err != nil {
		return nil, err
	}
	db, tenantID, err := s.tenantDatabase(ctx, "create a parent conference")
	if err != nil {
		return nil, err
	}
	row := &parentConferenceRow{TenantID: tenantID}
	applyParentConferenceFields(row, fields)
	if err := db.NewInsert().Model(row).ModelTableExpr(`calendar.parent_conferences`).Returning("*").Scan(ctx); err != nil {
		return nil, fmt.Errorf("school calendar postgres: create parent conference: %w", err)
	}
	return parentConferenceToPublic(*row), nil
}

func (s ParentConferences) UpdateParentConference(ctx context.Context, id int64, fields schoolcalendar.ParentConferenceFields) (*schoolcalendar.ParentConference, error) {
	fields, err := schoolcalendar.ValidateParentConferenceFields(fields)
	if err != nil {
		return nil, err
	}
	db, tenantID, err := s.tenantDatabase(ctx, "update a parent conference")
	if err != nil {
		return nil, err
	}
	row := &parentConferenceRow{ID: id, TenantID: tenantID}
	applyParentConferenceFields(row, fields)
	err = db.NewUpdate().Model(row).ModelTableExpr(parentConferenceTable).
		Column("school_class", "conference_date", "start_minute", "end_minute", "slot_minutes", "breaks", "location").
		Where(`"parent_conference".tenant_id = ?`, tenantID).
		Where(`"parent_conference".id = ?`, id).
		Returning("*").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, schoolcalendar.ErrParentConferenceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("school calendar postgres: update parent conference: %w", err)
	}
	return parentConferenceToPublic(*row), nil
}

func (s ParentConferences) FindParentConference(ctx context.Context, id int64, lock bool) (*schoolcalendar.ParentConference, error) {
	if id <= 0 {
		return nil, schoolcalendar.ErrParentConferenceNotFound
	}
	db, tenantID, err := s.tenantDatabase(ctx, "find a parent conference")
	if err != nil {
		return nil, err
	}
	row := new(parentConferenceRow)
	query := db.NewSelect().Model(row).ModelTableExpr(parentConferenceTable).
		Where(`"parent_conference".tenant_id = ?`, tenantID).
		Where(`"parent_conference".id = ?`, id)
	if lock {
		query = query.For("UPDATE")
	}
	err = query.Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, schoolcalendar.ErrParentConferenceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("school calendar postgres: find parent conference: %w", err)
	}
	return parentConferenceToPublic(*row), nil
}

func (s ParentConferences) ListParentConferences(ctx context.Context, filter schoolcalendar.ParentConferenceFilter) ([]*schoolcalendar.ParentConference, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "list parent conferences")
	if err != nil {
		return nil, err
	}
	rows := []parentConferenceRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(parentConferenceTable).
		Where(`"parent_conference".tenant_id = ?`, tenantID).
		OrderExpr(`"parent_conference".conference_date, "parent_conference".start_minute, "parent_conference".id`)
	if filter.OrganizerStaffID > 0 {
		query = query.Where(`"parent_conference".organizer_staff_id = ?`, filter.OrganizerStaffID)
	}
	if !filter.From.IsZero() {
		query = query.Where(`"parent_conference".conference_date >= ?`, filter.From)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("school calendar postgres: list parent conferences: %w", err)
	}
	result := make([]*schoolcalendar.ParentConference, 0, len(rows))
	for _, row := range rows {
		result = append(result, parentConferenceToPublic(row))
	}
	return result, nil
}

// DeleteParentConference deletes the day; its bookings cascade. The caller
// cancels the bookings' appointments first.
func (s ParentConferences) DeleteParentConference(ctx context.Context, id int64) error {
	db, tenantID, err := s.tenantDatabase(ctx, "delete a parent conference")
	if err != nil {
		return err
	}
	result, err := db.NewDelete().Model((*parentConferenceRow)(nil)).ModelTableExpr(parentConferenceTable).
		Where(`"parent_conference".tenant_id = ?`, tenantID).
		Where(`"parent_conference".id = ?`, id).Exec(ctx)
	return affectedOne(result, err, "delete parent conference")
}

func (s ParentConferences) ListParentConferenceBookings(ctx context.Context, conferenceIDs []int64) ([]*schoolcalendar.ParentConferenceBooking, error) {
	if len(conferenceIDs) == 0 {
		return []*schoolcalendar.ParentConferenceBooking{}, nil
	}
	db, tenantID, err := s.tenantDatabase(ctx, "list parent conference bookings")
	if err != nil {
		return nil, err
	}
	rows := []parentBookingRow{}
	err = db.NewSelect().Model(&rows).ModelTableExpr(parentBookingTable).
		Where(`"parent_booking".tenant_id = ?`, tenantID).
		Where(`"parent_booking".conference_id IN (?)`, bun.List(conferenceIDs)).
		OrderExpr(`"parent_booking".conference_id, "parent_booking".slot_start_minute`).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("school calendar postgres: list parent conference bookings: %w", err)
	}
	result := make([]*schoolcalendar.ParentConferenceBooking, 0, len(rows))
	for _, row := range rows {
		result = append(result, parentBookingToPublic(row))
	}
	return result, nil
}

func (s ParentConferences) CreateParentConferenceBooking(ctx context.Context, booking schoolcalendar.ParentConferenceBooking) (*schoolcalendar.ParentConferenceBooking, error) {
	if booking.ConferenceID <= 0 || booking.StudentID <= 0 || booking.SlotStartMinute < 0 {
		return nil, fmt.Errorf("%w: parent conference, child and slot are required", schoolcalendar.ErrInvalidParentConference)
	}
	db, tenantID, err := s.tenantDatabase(ctx, "book a parent conference slot")
	if err != nil {
		return nil, err
	}
	row := &parentBookingRow{
		TenantID: tenantID, ConferenceID: booking.ConferenceID, StudentID: booking.StudentID,
		GuardianProfileID: booking.GuardianProfileID, SlotStartMinute: booking.SlotStartMinute,
		AppointmentID: booking.AppointmentID,
	}
	if err := db.NewInsert().Model(row).ModelTableExpr(`calendar.parent_conference_bookings`).Returning("*").Scan(ctx); err != nil {
		return nil, classifyParentBookingConflict(fmt.Errorf("school calendar postgres: book parent conference slot: %w", err))
	}
	return parentBookingToPublic(*row), nil
}

func (s ParentConferences) MoveParentConferenceBooking(ctx context.Context, bookingID int64, slotStartMinute int) error {
	db, tenantID, err := s.tenantDatabase(ctx, "move a parent conference booking")
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().Model((*parentBookingRow)(nil)).ModelTableExpr(parentBookingTable).
		Set("slot_start_minute = ?", slotStartMinute).
		Where(`"parent_booking".tenant_id = ?`, tenantID).
		Where(`"parent_booking".id = ?`, bookingID).Exec(ctx)
	return classifyParentBookingConflict(affectedOne(result, err, "move parent conference booking"))
}

func (s ParentConferences) LinkParentConferenceBookingAppointment(ctx context.Context, bookingID, appointmentID int64) error {
	db, tenantID, err := s.tenantDatabase(ctx, "link a parent conference booking")
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().Model((*parentBookingRow)(nil)).ModelTableExpr(parentBookingTable).
		Set("appointment_id = ?", appointmentID).
		Where(`"parent_booking".tenant_id = ?`, tenantID).
		Where(`"parent_booking".id = ?`, bookingID).Exec(ctx)
	return affectedOne(result, err, "link parent conference booking")
}

func (s ParentConferences) DeleteParentConferenceBookings(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	db, tenantID, err := s.tenantDatabase(ctx, "delete parent conference bookings")
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*parentBookingRow)(nil)).ModelTableExpr(parentBookingTable).
		Where(`"parent_booking".tenant_id = ?`, tenantID).
		Where(`"parent_booking".id IN (?)`, bun.List(ids)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("school calendar postgres: delete parent conference bookings: %w", err)
	}
	return nil
}

// DeleteParentConferencesBefore deletes the days of the past school year
// with their bookings.
func (s ParentConferences) DeleteParentConferencesBefore(ctx context.Context, cutoff calendar.Date) (int, error) {
	if cutoff.IsZero() {
		return 0, fmt.Errorf("%w: cutoff is required", schoolcalendar.ErrInvalidParentConference)
	}
	db, tenantID, err := s.tenantDatabase(ctx, "delete past parent conferences")
	if err != nil {
		return 0, err
	}
	var bookings int
	err = db.NewSelect().TableExpr(parentBookingTable).ColumnExpr("COUNT(*)").
		Join(`JOIN calendar.parent_conferences AS "parent_conference" ON "parent_conference".tenant_id = "parent_booking".tenant_id AND "parent_conference".id = "parent_booking".conference_id`).
		Where(`"parent_booking".tenant_id = ?`, tenantID).
		Where(`"parent_conference".conference_date < ?`, cutoff).Scan(ctx, &bookings)
	if err != nil {
		return 0, fmt.Errorf("school calendar postgres: count past parent conference bookings: %w", err)
	}
	result, err := db.NewDelete().Model((*parentConferenceRow)(nil)).ModelTableExpr(parentConferenceTable).
		Where(`"parent_conference".tenant_id = ?`, tenantID).
		Where(`"parent_conference".conference_date < ?`, cutoff).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("school calendar postgres: delete past parent conferences: %w", err)
	}
	days, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("school calendar postgres: count deleted parent conferences: %w", err)
	}
	return bookings + int(days), nil
}

// affectedOne reports a write that matched no row as not found.
func affectedOne(result sql.Result, err error, operation string) error {
	if err != nil {
		return fmt.Errorf("school calendar postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("school calendar postgres: %s count rows: %w", operation, err)
	}
	if rows == 0 {
		return schoolcalendar.ErrParentConferenceNotFound
	}
	return nil
}

// classifyParentBookingConflict names the double-booking constraint a write
// hit and keeps the driver error in the chain.
func classifyParentBookingConflict(err error) error {
	var postgresError pgdriver.Error
	if err == nil || !errors.As(err, &postgresError) || !postgresError.IntegrityViolation() {
		return err
	}
	switch postgresError.Field('n') {
	case parentBookingSlotConstraint:
		return fmt.Errorf("%w: %w", schoolcalendar.ErrParentConferenceSlotTaken, err)
	case parentBookingStudentConstraint:
		return fmt.Errorf("%w: %w", schoolcalendar.ErrParentConferenceStudentBooked, err)
	default:
		return err
	}
}

func applyParentConferenceFields(row *parentConferenceRow, fields schoolcalendar.ParentConferenceFields) {
	row.OrganizerStaffID = fields.OrganizerStaffID
	row.OrganizerAccountID = fields.OrganizerAccountID
	row.SchoolClass = fields.SchoolClass
	row.ConferenceDate = fields.Date
	row.StartMinute = fields.StartMinute
	row.EndMinute = fields.EndMinute
	row.SlotMinutes = fields.SlotMinutes
	row.Breaks = fields.Breaks
	if row.Breaks == nil {
		row.Breaks = []schoolcalendar.ParentConferenceBreak{}
	}
	row.Location = fields.Location
}

func parentConferenceToPublic(row parentConferenceRow) *schoolcalendar.ParentConference {
	breaks := row.Breaks
	if breaks == nil {
		breaks = []schoolcalendar.ParentConferenceBreak{}
	}
	return &schoolcalendar.ParentConference{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ParentConferenceFields: schoolcalendar.ParentConferenceFields{
			OrganizerStaffID: row.OrganizerStaffID, OrganizerAccountID: row.OrganizerAccountID,
			SchoolClass: row.SchoolClass, Date: row.ConferenceDate,
			StartMinute: row.StartMinute, EndMinute: row.EndMinute, SlotMinutes: row.SlotMinutes,
			Breaks: breaks, Location: row.Location,
		},
	}
}

func parentBookingToPublic(row parentBookingRow) *schoolcalendar.ParentConferenceBooking {
	return &schoolcalendar.ParentConferenceBooking{
		ID: row.ID, TenantID: row.TenantID, ConferenceID: row.ConferenceID, StudentID: row.StudentID,
		GuardianProfileID: row.GuardianProfileID, SlotStartMinute: row.SlotStartMinute,
		AppointmentID: row.AppointmentID, CreatedAt: row.CreatedAt,
	}
}
