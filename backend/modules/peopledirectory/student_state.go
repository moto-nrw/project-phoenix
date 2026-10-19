package peopledirectory

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentState is a child as the student routes carry it: People Directory's owned
// row (peopledirectory.StudentRecord, #3349) with its calendar dates as dates,
// the departure plan resolved into its effective form, and the two pieces of
// bookkeeping a write needs. The routes read and write it through the owner's
// capability; nothing here decides anything the owner decides.
type StudentState struct {
	Person    *DirectoryPersonSnapshot `json:"person,omitempty"`
	ID        int64                    `json:"id"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
	TenantID  int64                    `json:"tenant_id"`

	PersonID          int64   `json:"person_id"`
	SchoolClass       string  `json:"school_class"`
	GroupID           *int64  `json:"group_id,omitempty"`
	AddressStreet     *string `json:"address_street,omitempty"`
	AddressCity       *string `json:"address_city,omitempty"`
	AddressPostalCode *string `json:"address_postal_code,omitempty"`
	ExtraInfo         *string `json:"extra_info,omitempty"`
	HealthInfo        *string `json:"health_info,omitempty"`
	PickupStatus      *string `json:"pickup_status,omitempty"`

	// DepartureDays, AllowedDepartureModes, PickupDays and BusDays are the
	// departure plan in its effective form (departure.Plan.Effective): the
	// unified per-weekday modes and the two legacy projections derived from
	// them.
	DepartureDays          departure.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes  departure.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	PickupDays             departure.PickupDays            `json:"pickup_days,omitempty"`
	BusDays                departure.BusDays               `json:"bus_days,omitempty"`
	DepartureCompanionNote *string                         `json:"departure_companion_note,omitempty"`
	// DepartureCompanionDays carries the weekdays on which the child has a
	// companion link after this request's reconcile. It is not persisted.
	DepartureCompanionDays map[string]bool `json:"-"`
	// DepartureBaseline is the plan the read that produced this value
	// hydrated. A write hands it to the owner, so a plan field that merely rode
	// along on the read cannot revert a companion edit that committed in
	// between (#1694). Nil means "not hydrated".
	DepartureBaseline *departure.Plan `json:"-"`

	Sick          *bool          `json:"sick,omitempty"`
	SickSince     *time.Time     `json:"sick_since,omitempty"`
	Excused       *bool          `json:"excused,omitempty"`
	ExcusedSince  *time.Time     `json:"excused_since,omitempty"`
	Status        string         `json:"status"`
	EnrolledFrom  *calendar.Date `json:"enrolled_from,omitempty"`
	EnrolledUntil *calendar.Date `json:"enrolled_until,omitempty"`

	PhotoPath           *string    `json:"-"`
	PhotoConsentGivenAt *time.Time `json:"photo_consent_given_at,omitempty"`
	PhotoConsentGivenBy *int64     `json:"photo_consent_given_by,omitempty"`

	AGBAcceptedAt            *time.Time `json:"agb_accepted_at,omitempty"`
	DataProcessingAcceptedAt *time.Time `json:"data_processing_accepted_at,omitempty"`
	EmailContactAcceptedAt   *time.Time `json:"email_contact_accepted_at,omitempty"`
}

// IsAuthorizationStudent reports whether this value represents an existing
// child. Nil-safe for the Security Runtime gates.
func (s *StudentState) IsAuthorizationStudent() bool { return s != nil }

// IsAlumnus reports whether the child graduated. A nil row is not an alumnus.
func (s *StudentState) IsAlumnus() bool {
	return s != nil && s.Status == StudentStatusAlumnus
}

// CareEndedOn reports whether the child's care ended before the given day;
// the last care day itself still counts as care.
func (s *StudentState) CareEndedOn(day calendar.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}

// CareLifecycle is a child's graduation and end-of-care facts, as every
// student view of the People Directory reports them.
type CareLifecycle interface {
	IsAlumnus() bool
	CareEndedOn(day calendar.Date) bool
}

// AttendsOn reports whether a child still attends on day: a graduate does
// not, nor a child past the last care day (#405, #2487); the last care day
// itself still counts.
func AttendsOn(student CareLifecycle, day calendar.Date) bool {
	return !student.IsAlumnus() && !student.CareEndedOn(day)
}

// CareEndsLater reports whether an end of care is recorded but has not taken
// effect yet on the given day.
func (s *StudentState) CareEndsLater(day calendar.Date) bool {
	return s != nil && s.EnrolledUntil != nil && !day.After(*s.EnrolledUntil)
}

// SnapshotDeparturePlan records the current in-memory departure plan as the
// hydrated baseline. Normalizing copies the maps, so a later in-place edit of
// the plan cannot move the baseline with it.
func (s *StudentState) SnapshotDeparturePlan() {
	s.DepartureBaseline = &departure.Plan{
		DepartureDays:         s.DepartureDays.Normalize(),
		AllowedDepartureModes: s.AllowedDepartureModes.Normalize(),
		BusDays:               s.BusDays.Normalize(),
		PickupDays:            s.PickupDays.Normalize(),
	}
}

// StudentStateFromRecord builds the route's view of an owned row. The calendar
// dates travel as YYYY-MM-DD; a malformed one would be an owner bug rather
// than input, so it becomes an unset date instead of a wrong day.
func StudentStateFromRecord(record StudentRecord) *StudentState {
	student := &StudentState{
		ID: record.ID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, TenantID: record.TenantID,

		PersonID: record.PersonID, SchoolClass: record.SchoolClass, GroupID: record.GroupID,
		Status: record.Status,

		AddressStreet:     record.AddressStreet,
		AddressCity:       record.AddressCity,
		AddressPostalCode: record.AddressPostalCode,

		ExtraInfo:    record.ExtraInfo,
		HealthInfo:   record.HealthInfo,
		PickupStatus: record.PickupStatus,

		DepartureDays:          record.DepartureDays,
		AllowedDepartureModes:  record.AllowedDepartureModes,
		PickupDays:             record.PickupDays,
		BusDays:                record.BusDays,
		DepartureCompanionNote: record.DepartureCompanionNote,

		Sick:         record.Sick,
		SickSince:    record.SickSince,
		Excused:      record.Excused,
		ExcusedSince: record.ExcusedSince,

		EnrolledFrom:  OptionalCalendarDate(record.EnrolledFrom),
		EnrolledUntil: OptionalCalendarDate(record.EnrolledUntil),

		PhotoPath:           record.PhotoPath,
		PhotoConsentGivenAt: record.PhotoConsentGivenAt,
		PhotoConsentGivenBy: record.PhotoConsentGivenBy,

		AGBAcceptedAt:            record.AGBAcceptedAt,
		DataProcessingAcceptedAt: record.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   record.EmailContactAcceptedAt,
	}
	student.HydrateDeparturePlan()
	return student
}

// StudentStatesFromRecords maps owned rows in order.
func StudentStatesFromRecords(records []StudentRecord) []*StudentState {
	students := make([]*StudentState, 0, len(records))
	for _, record := range records {
		students = append(students, StudentStateFromRecord(record))
	}
	return students
}

// StudentStatesByID keys owned rows by id.
func StudentStatesByID(records []StudentRecord) map[int64]*StudentState {
	students := make(map[int64]*StudentState, len(records))
	for _, record := range records {
		students[record.ID] = StudentStateFromRecord(record)
	}
	return students
}

// hydrateDeparturePlan resolves the stored departure projections into the one
// effective plan and records it as the update baseline. The precedence is the
// owner's (departure.Plan.Effective).
func (s *StudentState) HydrateDeparturePlan() {
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
func (s *StudentState) DirectoryRecord() StudentRecord {
	return StudentRecord{
		ID: s.ID, TenantID: s.TenantID,
		PersonID: s.PersonID, SchoolClass: s.SchoolClass,
		GroupID: s.GroupID, Status: s.Status,

		EnrolledFrom:  RenderCalendarDate(s.EnrolledFrom),
		EnrolledUntil: RenderCalendarDate(s.EnrolledUntil),

		AddressStreet:     s.AddressStreet,
		AddressCity:       s.AddressCity,
		AddressPostalCode: s.AddressPostalCode,

		ExtraInfo:    s.ExtraInfo,
		HealthInfo:   s.HealthInfo,
		PickupStatus: s.PickupStatus,

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
func (s *StudentState) DirectoryWrite() StudentWrite {
	write := StudentWrite{
		Record: s.DirectoryRecord(),
		Plan: StudentPlan{
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
		write.Baseline = &StudentPlan{
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
func (s *StudentState) ApplyDirectoryRecord(record StudentRecord) {
	s.ID, s.CreatedAt, s.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	s.TenantID = record.TenantID
	s.PersonID, s.SchoolClass = record.PersonID, record.SchoolClass
	s.GroupID, s.Status = record.GroupID, record.Status
	s.EnrolledFrom = OptionalCalendarDate(record.EnrolledFrom)
	s.EnrolledUntil = OptionalCalendarDate(record.EnrolledUntil)

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
func (s *StudentState) AuditSnapshot() StudentAuditSnapshot {
	return StudentAuditSnapshot{
		StudentID:              s.ID,
		Status:                 s.Status,
		CareEnd:                RenderCalendarDate(s.EnrolledUntil),
		ExtraInfo:              s.ExtraInfo,
		HealthInfo:             s.HealthInfo,
		PickupStatus:           s.PickupStatus,
		DepartureCompanionNote: s.DepartureCompanionNote,
		AllowedDepartureModes:  s.AllowedDepartureModes,
		DepartureDays:          s.DepartureDays,
	}
}

// consentSnapshot projects the child onto the four consent timestamps.
func (s *StudentState) ConsentSnapshot() StudentConsentSnapshot {
	return StudentConsentSnapshot{
		StudentID:                s.ID,
		AGBAcceptedAt:            s.AGBAcceptedAt,
		DataProcessingAcceptedAt: s.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   s.EmailContactAcceptedAt,
		PhotoConsentGivenAt:      s.PhotoConsentGivenAt,
	}
}

// IsMissingStudent reports a child the owner would not hand over: no such row,
// or an unusable id. Both mean "no such child of this tenant".
func IsMissingStudent(err error) bool {
	return errors.Is(err, ErrStudentNotFound) || errors.Is(err, ErrInvalidStudent)
}

// TranslateStudentWriteError restates the owner's companion refusals in the
// departure contract's user-facing sentinels, whose German text the routes
// render as is.
func TranslateStudentWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrCompanionWouldLoseDeparture):
		return departure.ErrCompanionWouldLoseDeparture
	case errors.Is(err, ErrCompanionLockBusy):
		return departure.ErrCompanionLockBusy
	default:
		return err
	}
}

func OptionalCalendarDate(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func RenderCalendarDate(value *calendar.Date) string {
	if value == nil {
		return ""
	}
	return value.String()
}
