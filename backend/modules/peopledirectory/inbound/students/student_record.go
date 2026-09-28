package students

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// The lifecycle states of a child as People Directory stores them.
const (
	studentStatusPending  = peopleModule.StudentStatusPending
	studentStatusActive   = peopleModule.StudentStatusActive
	studentStatusInactive = "inactive"
	studentStatusAlumnus  = peopleModule.StudentStatusAlumnus
)

// Student is a child as the student routes carry it: People Directory's owned
// row (peopledirectory.StudentRecord, #3349) with its calendar dates as dates,
// the departure plan resolved into its effective form, and the two pieces of
// bookkeeping a write needs. The routes read and write it through the owner's
// capability; nothing here decides anything the owner decides.
type Student struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	TenantID  int64

	PersonID          int64
	SchoolClass       string
	GroupID           *int64
	AddressStreet     *string
	AddressCity       *string
	AddressPostalCode *string
	ExtraInfo         *string
	SupervisorNotes   *string
	HealthInfo        *string
	PickupStatus      *string

	// DepartureDays, AllowedDepartureModes, PickupDays and BusDays are the
	// departure plan in its effective form (departure.Plan.Effective): the
	// unified per-weekday modes and the two legacy projections derived from
	// them.
	DepartureDays          departure.DepartureDays
	AllowedDepartureModes  departure.AllowedDepartureModes
	PickupDays             departure.PickupDays
	BusDays                departure.BusDays
	DepartureCompanionNote *string
	// DepartureCompanionDays carries the weekdays on which the child has a
	// companion link after this request's reconcile. It is not persisted.
	DepartureCompanionDays map[string]bool
	// DepartureBaseline is the plan the read that produced this value
	// hydrated. A write hands it to the owner, so a plan field that merely rode
	// along on the read cannot revert a companion edit that committed in
	// between (#1694). Nil means "not hydrated".
	DepartureBaseline *departure.Plan

	Sick          *bool
	SickSince     *time.Time
	Excused       *bool
	ExcusedSince  *time.Time
	Status        string
	EnrolledFrom  *timezone.Date
	EnrolledUntil *timezone.Date

	PhotoPath           *string
	PhotoConsentGivenAt *time.Time
	PhotoConsentGivenBy *int64

	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
}

// IsAuthorizationStudent reports whether this value represents an existing
// child. Nil-safe for the Security Runtime gates.
func (s *Student) IsAuthorizationStudent() bool { return s != nil }

// IsAlumnus reports whether the child graduated. A nil row is not an alumnus.
func (s *Student) IsAlumnus() bool {
	return s != nil && s.Status == studentStatusAlumnus
}

// CareEndedOn reports whether the child's care ended before the given day;
// the last care day itself still counts as care.
func (s *Student) CareEndedOn(day timezone.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}

// CareEndsLater reports whether an end of care is recorded but has not taken
// effect yet on the given day.
func (s *Student) CareEndsLater(day timezone.Date) bool {
	return s != nil && s.EnrolledUntil != nil && !day.After(*s.EnrolledUntil)
}

// SnapshotDeparturePlan records the current in-memory departure plan as the
// hydrated baseline. Normalizing copies the maps, so a later in-place edit of
// the plan cannot move the baseline with it.
func (s *Student) SnapshotDeparturePlan() {
	s.DepartureBaseline = &departure.Plan{
		DepartureDays:         s.DepartureDays.Normalize(),
		AllowedDepartureModes: s.AllowedDepartureModes.Normalize(),
		BusDays:               s.BusDays.Normalize(),
		PickupDays:            s.PickupDays.Normalize(),
	}
}

// studentFromRecord builds the route's view of an owned row. The calendar
// dates travel as YYYY-MM-DD; a malformed one would be an owner bug rather
// than input, so it becomes an unset date instead of a wrong day.
func studentFromRecord(record peopleModule.StudentRecord) *Student {
	student := &Student{
		ID: record.ID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, TenantID: record.TenantID,

		PersonID: record.PersonID, SchoolClass: record.SchoolClass, GroupID: record.GroupID,
		Status: record.Status,

		AddressStreet:     record.AddressStreet,
		AddressCity:       record.AddressCity,
		AddressPostalCode: record.AddressPostalCode,

		ExtraInfo:       record.ExtraInfo,
		SupervisorNotes: record.SupervisorNotes,
		HealthInfo:      record.HealthInfo,
		PickupStatus:    record.PickupStatus,

		DepartureDays:          record.DepartureDays,
		AllowedDepartureModes:  record.AllowedDepartureModes,
		PickupDays:             record.PickupDays,
		BusDays:                record.BusDays,
		DepartureCompanionNote: record.DepartureCompanionNote,

		Sick:         record.Sick,
		SickSince:    record.SickSince,
		Excused:      record.Excused,
		ExcusedSince: record.ExcusedSince,

		EnrolledFrom:  optionalCalendarDate(record.EnrolledFrom),
		EnrolledUntil: optionalCalendarDate(record.EnrolledUntil),

		PhotoPath:           record.PhotoPath,
		PhotoConsentGivenAt: record.PhotoConsentGivenAt,
		PhotoConsentGivenBy: record.PhotoConsentGivenBy,

		AGBAcceptedAt:            record.AGBAcceptedAt,
		DataProcessingAcceptedAt: record.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   record.EmailContactAcceptedAt,
	}
	student.hydrateDeparturePlan()
	return student
}

// studentsFromRecords maps owned rows in order.
func studentsFromRecords(records []peopleModule.StudentRecord) []*Student {
	students := make([]*Student, 0, len(records))
	for _, record := range records {
		students = append(students, studentFromRecord(record))
	}
	return students
}

// studentsByID keys owned rows by id.
func studentsByID(records []peopleModule.StudentRecord) map[int64]*Student {
	students := make(map[int64]*Student, len(records))
	for _, record := range records {
		students[record.ID] = studentFromRecord(record)
	}
	return students
}

// hydrateDeparturePlan resolves the stored departure projections into the one
// effective plan and records it as the update baseline. The precedence is the
// owner's (departure.Plan.Effective).
func (s *Student) hydrateDeparturePlan() {
	effective := departure.Plan{
		AllowedDepartureModes: s.AllowedDepartureModes,
		DepartureDays:         s.DepartureDays,
		BusDays:               s.BusDays,
		PickupDays:            s.PickupDays,
	}.Effective()
	s.AllowedDepartureModes = effective.AllowedDepartureModes
	s.DepartureDays = effective.DepartureDays
	s.BusDays = effective.BusDays
	s.PickupDays = effective.PickupDays
	s.SnapshotDeparturePlan()
}

// record projects the child onto the owner's row.
func (s *Student) record() peopleModule.StudentRecord {
	return peopleModule.StudentRecord{
		ID: s.ID, TenantID: s.TenantID,
		PersonID: s.PersonID, SchoolClass: s.SchoolClass,
		GroupID: s.GroupID, Status: s.Status,

		EnrolledFrom:  renderCalendarDate(s.EnrolledFrom),
		EnrolledUntil: renderCalendarDate(s.EnrolledUntil),

		AddressStreet:     s.AddressStreet,
		AddressCity:       s.AddressCity,
		AddressPostalCode: s.AddressPostalCode,

		ExtraInfo:       s.ExtraInfo,
		SupervisorNotes: s.SupervisorNotes,
		HealthInfo:      s.HealthInfo,
		PickupStatus:    s.PickupStatus,

		DepartureCompanionNote: s.DepartureCompanionNote,

		Sick:         s.Sick,
		SickSince:    s.SickSince,
		Excused:      s.Excused,
		ExcusedSince: s.ExcusedSince,

		PhotoPath:           s.PhotoPath,
		PhotoConsentGivenAt: s.PhotoConsentGivenAt,
		PhotoConsentGivenBy: s.PhotoConsentGivenBy,

		AGBAcceptedAt:            s.AGBAcceptedAt,
		DataProcessingAcceptedAt: s.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   s.EmailContactAcceptedAt,
	}
}

// write is the owner command for persisting this child: the row, the plan it
// carries and, when the read hydrated one, the baseline that plan started
// from.
func (s *Student) write() peopleModule.StudentWrite {
	write := peopleModule.StudentWrite{
		Record: s.record(),
		Plan: peopleModule.StudentPlan{
			AllowedDepartureModes: s.AllowedDepartureModes,
			DepartureDays:         s.DepartureDays,
			BusDays:               s.BusDays,
			PickupDays:            s.PickupDays,
			PickupStatus:          s.PickupStatus,
		},
		CompanionNote: s.DepartureCompanionNote,
		NoteSupplied:  s.DepartureCompanionNote != nil,
	}
	if baseline := s.DepartureBaseline; baseline != nil {
		write.Baseline = &peopleModule.StudentPlan{
			AllowedDepartureModes: baseline.AllowedDepartureModes,
			DepartureDays:         baseline.DepartureDays,
			BusDays:               baseline.BusDays,
			PickupDays:            baseline.PickupDays,
		}
	}
	return write
}

// applyRecord writes the stored row back onto the child and records the
// persisted plan as the new baseline. Field by field on purpose: the value
// also carries what the row does not (the companion days this request
// marked), and a write must not take those away.
func (s *Student) applyRecord(record peopleModule.StudentRecord) {
	s.ID, s.CreatedAt, s.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	s.TenantID = record.TenantID
	s.PersonID, s.SchoolClass = record.PersonID, record.SchoolClass
	s.GroupID, s.Status = record.GroupID, record.Status
	s.EnrolledFrom = optionalCalendarDate(record.EnrolledFrom)
	s.EnrolledUntil = optionalCalendarDate(record.EnrolledUntil)

	s.AllowedDepartureModes = record.AllowedDepartureModes
	s.DepartureDays = record.DepartureDays
	s.BusDays = record.BusDays
	s.PickupDays = record.PickupDays
	s.PickupStatus = record.PickupStatus
	s.DepartureCompanionNote = record.DepartureCompanionNote

	s.SnapshotDeparturePlan()
}

// auditSnapshot projects the child onto the owner's tracked change-history
// slice. CareEnd travels as a calendar date so the owner renders it.
func (s *Student) auditSnapshot() peopleModule.StudentAuditSnapshot {
	return peopleModule.StudentAuditSnapshot{
		StudentID:              s.ID,
		Status:                 s.Status,
		CareEnd:                renderCalendarDate(s.EnrolledUntil),
		SupervisorNotes:        s.SupervisorNotes,
		ExtraInfo:              s.ExtraInfo,
		HealthInfo:             s.HealthInfo,
		PickupStatus:           s.PickupStatus,
		DepartureCompanionNote: s.DepartureCompanionNote,
		AllowedDepartureModes:  s.AllowedDepartureModes,
		DepartureDays:          s.DepartureDays,
	}
}

// consentSnapshot projects the child onto the four consent timestamps.
func (s *Student) consentSnapshot() peopleModule.StudentConsentSnapshot {
	return peopleModule.StudentConsentSnapshot{
		StudentID:                s.ID,
		AGBAcceptedAt:            s.AGBAcceptedAt,
		DataProcessingAcceptedAt: s.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   s.EmailContactAcceptedAt,
		PhotoConsentGivenAt:      s.PhotoConsentGivenAt,
	}
}

// presenceRecord projects the child onto the Student Presence status-day
// write's view of it.
func (s *Student) presenceRecord() *studentpresence.StudentRecord {
	if s == nil {
		return nil
	}
	return &studentpresence.StudentRecord{
		ID:            s.ID,
		TenantID:      s.TenantID,
		PersonID:      s.PersonID,
		GroupID:       s.GroupID,
		SchoolClass:   s.SchoolClass,
		Lifecycle:     studentLifecycle(s.Status),
		EnrolledFrom:  s.EnrolledFrom,
		EnrolledUntil: s.EnrolledUntil,
		Sick:          s.Sick,
		SickSince:     s.SickSince,
		Excused:       s.Excused,
		ExcusedSince:  s.ExcusedSince,
	}
}

// studentLifecycle maps the owner's lifecycle status onto the states the
// presence flows branch on. Every other status, today only "pending", is
// StudentLifecycleOther and counts as not-yet-active.
func studentLifecycle(status string) studentpresence.StudentLifecycle {
	switch status {
	case studentStatusActive:
		return studentpresence.StudentLifecycleActive
	case studentStatusInactive:
		return studentpresence.StudentLifecycleInactive
	case studentStatusAlumnus:
		return studentpresence.StudentLifecycleAlumnus
	default:
		return studentpresence.StudentLifecycleOther
	}
}

// isMissingStudent reports a child the owner would not hand over: no such row,
// or an unusable id. Both mean "no such child of this tenant".
func isMissingStudent(err error) bool {
	return errors.Is(err, peopleModule.ErrStudentNotFound) || errors.Is(err, peopleModule.ErrInvalidStudent)
}

// translateStudentWriteError restates the owner's companion refusals in the
// departure contract's user-facing sentinels, whose German text the routes
// render as is.
func translateStudentWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, peopleModule.ErrCompanionWouldLoseDeparture):
		return departure.ErrCompanionWouldLoseDeparture
	case errors.Is(err, peopleModule.ErrCompanionLockBusy):
		return departure.ErrCompanionLockBusy
	default:
		return err
	}
}

func optionalCalendarDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	parsed, err := timezone.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func renderCalendarDate(value *timezone.Date) string {
	if value == nil {
		return ""
	}
	return value.String()
}
