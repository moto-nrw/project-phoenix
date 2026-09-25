package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Care Plan's booking materialization (#3560) reads and writes the Timetable,
// Enrollment, People Directory and Audit Platform through its own ports. The
// adapters below bind them over the owners' public capabilities with the
// queries the retained repositories issued; they decide nothing.

// bookingMaterializationInputs are the owners the materialization is composed
// over.
type bookingMaterializationInputs struct {
	Catalog     careplan.CareOfferingCatalogCapability
	Timetable   bookingTimetableOwner
	Rosters     bookingRosterMaintenance
	Students    bookingStudentOwner
	Periods     bookingPeriodReads
	Enrollment  bookingEnrollmentReads
	Approved    bookingApprovedChildren
	Settings    bookingSettingsReads
	Bookings    careplanCompose.BookingCommands
	Withdrawals careplanCompose.BookingWithdrawals
	Adjustments auditModels.EnrollmentOfferingAdjustmentRepository
	Persons     bookingActorPersons
	Accounts    bookingActorAccounts
	Pickup      bookingPickupReads
	PickupRows  bookingPickupRecords

	LockRecurrence              func(context.Context) error
	ResyncPickupAutoExcusals    func(ctx context.Context, studentIDs []int64) error
	ClearPickupWeekdayExtension func(ctx context.Context, studentID int64, weekday int) error
	Broadcaster                 realtime.Broadcaster
	GuardianNotifier            bookingGuardianNotifier
	Today                       func() calendar.Date
	Logger                      *slog.Logger
}

func newBookingMaterialization(inputs bookingMaterializationInputs) (careplan.BookingMaterializationCapability, error) {
	if inputs.Timetable == nil || inputs.Rosters == nil || inputs.Students == nil || inputs.Enrollment == nil ||
		inputs.Approved == nil || inputs.Adjustments == nil || inputs.Pickup == nil || inputs.PickupRows == nil {
		return nil, errors.New("booking materialization: timetable, rosters, students, enrollment, approved bookings, adjustment audit and pickup are required")
	}
	logger := inputs.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return careplanCompose.NewBookingMaterialization(careplanCompose.BookingMaterializationDependencies{
		Catalog:     inputs.Catalog,
		Rosters:     bookingRosters{timetable: inputs.Timetable, students: inputs.Students, rosters: inputs.Rosters},
		Templates:   bookingTemplates{timetable: inputs.Timetable},
		Periods:     bookingPeriods{calendar: inputs.Periods},
		Enrollment:  bookingEnrollment{owner: inputs.Enrollment, approved: inputs.Approved},
		Students:    bookingStudents{people: inputs.Students},
		Settings:    bookingSettings{settings: inputs.Settings},
		Bookings:    inputs.Bookings,
		Withdrawals: inputs.Withdrawals,
		Audit:       bookingAdjustmentAudit{repo: inputs.Adjustments, persons: inputs.Persons, accounts: inputs.Accounts},
		Pickup:      bookingPickup{baselines: inputs.Pickup, rows: inputs.PickupRows},

		LockTemplateRecurrence:      inputs.LockRecurrence,
		ResyncPickupAutoExcusals:    inputs.ResyncPickupAutoExcusals,
		ClearPickupWeekdayExtension: inputs.ClearPickupWeekdayExtension,
		AnnouncePickupChange:        announceOfferingPickupChange(inputs.Broadcaster, inputs.GuardianNotifier, logger),
		Today:                       inputs.Today,
		Logger:                      logger,
	})
}

// timetableOfferingRosterResync serves the Timetable owner's roster resync
// hook (#2137) from Care Plan's booking materialization.
func timetableOfferingRosterResync(rosters careplan.SourcedRosters) func(context.Context, timetable.OfferingRosterResyncInput) error {
	return func(ctx context.Context, in timetable.OfferingRosterResyncInput) error {
		return rosters.ResyncTemplateOfferingRoster(ctx, careplan.OfferingRosterResync{
			TemplateID: in.TemplateID, OfferingIDs: in.OfferingIDs, GradeLevels: in.GradeLevels,
			SchoolClasses: in.SchoolClasses, CalendarPeriodID: in.CalendarPeriodID, EffectiveFrom: in.EffectiveFrom,
			ScopeRequestChildIDs: in.ScopeRequestChildIDs, TolerateDriftedSources: in.TolerateDriftedSources,
		})
	}
}

// carePlanCareOfferings serves the catalog, the booking materialization it
// feeds, the offering-change review and the pickup adjustments as one Care
// Plan capability.
type carePlanCareOfferings struct {
	careplan.CareOfferingCatalogCapability
	careplan.BookingMaterializationCapability
	careplan.OfferingChangeCapability
	careplan.PickupAdjustments
}

type bookingTimetableOwner interface {
	ListStudentEnrollments(context.Context, timetable.StudentEnrollmentFilter) ([]timetable.StudentEnrollment, error)
	CreateStudentEnrollment(context.Context, timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error)
	DeleteStudentEnrollment(context.Context, int64) error
	SetStudentEnrollmentValidUntil(context.Context, int64, string) error
	DeleteStudentEnrollmentsBySource(context.Context, int64, int64) (int64, error)
	BackfillStudentEnrollmentSource(context.Context, int64, int64, []int64) (int64, error)
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error)
	UpdateGroupOfferingSource(context.Context, int64, timetable.OfferingSourceInput) error
}

type bookingRosterMaintenance interface {
	ReconcileSourcedTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from calendar.Date, prior []timetable.RosterEnrollment) (int, int, error)
}

type bookingStudentOwner interface {
	peopledirectory.StudentCommand
	ListStudentsByID(context.Context, []int64) ([]peopledirectory.Student, error)
}

// bookingRosters reads and writes activities.student_enrollments through the
// Timetable owner, with the filters the retained enrollment repository used.
type bookingRosters struct {
	timetable bookingTimetableOwner
	students  bookingStudentOwner
	rosters   bookingRosterMaintenance
}

func (r bookingRosters) GroupEnrollments(ctx context.Context, groupID int64) ([]careplanCompose.RosterEnrollment, error) {
	rows, err := r.list(ctx, timetable.StudentEnrollmentFilter{ActivityGroupIDs: []int64{groupID}, OrderByValidFrom: true})
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	students, err := r.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	alumni := make(map[int64]bool, len(students))
	for _, student := range students {
		alumni[student.ID] = student.IsAlumnus()
	}
	for i := range rows {
		rows[i].StudentAlumnus = alumni[rows[i].StudentID]
	}
	return rows, nil
}

func (r bookingRosters) StudentEnrollments(ctx context.Context, studentID int64) ([]careplanCompose.RosterEnrollment, error) {
	return r.list(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{studentID}, OrderByValidFrom: true})
}

func (r bookingRosters) list(ctx context.Context, filter timetable.StudentEnrollmentFilter) ([]careplanCompose.RosterEnrollment, error) {
	values, err := r.timetable.ListStudentEnrollments(ctx, filter)
	if err != nil {
		return nil, err
	}
	rows := make([]careplanCompose.RosterEnrollment, 0, len(values))
	for _, value := range values {
		rows = append(rows, careplanCompose.RosterEnrollment{
			ID: value.ID, StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
			ValidFrom: calendar.Date(value.ValidFrom), ValidUntil: optionalCalendarDate(value.ValidUntil),
			CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
			SelectedWeekdays: value.SelectedWeekdays, Weekday: value.Weekday,
		})
	}
	return rows, nil
}

func (r bookingRosters) CreateEnrollment(ctx context.Context, row careplanCompose.RosterEnrollment) error {
	var validUntil *string
	if row.ValidUntil != nil {
		text := row.ValidUntil.String()
		validUntil = &text
	}
	_, err := r.timetable.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: row.StudentID, ActivityGroupID: row.ActivityGroupID, ValidFrom: row.ValidFrom.String(), ValidUntil: validUntil,
		CalendarPeriodID: row.CalendarPeriodID, EnrollmentRequestChildID: row.EnrollmentRequestChildID,
		SelectedWeekdays: row.SelectedWeekdays, Weekday: row.Weekday,
	})
	return err
}

func (r bookingRosters) DeleteEnrollment(ctx context.Context, id int64) error {
	return r.timetable.DeleteStudentEnrollment(ctx, id)
}

func (r bookingRosters) SetEnrollmentValidUntil(ctx context.Context, id int64, validUntil calendar.Date) error {
	return r.timetable.SetStudentEnrollmentValidUntil(ctx, id, validUntil.String())
}

func (r bookingRosters) DeleteRequestChildEnrollments(ctx context.Context, studentID, requestChildID int64) error {
	if studentID <= 0 {
		return errors.New("student_id is required")
	}
	if requestChildID <= 0 {
		return errors.New("enrollment_request_child_id is required")
	}
	_, err := r.timetable.DeleteStudentEnrollmentsBySource(ctx, studentID, requestChildID)
	return err
}

func (r bookingRosters) BackfillRequestChildSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) error {
	if studentID <= 0 {
		return errors.New("student_id is required")
	}
	if requestChildID <= 0 {
		return errors.New("enrollment_request_child_id is required")
	}
	_, err := r.timetable.BackfillStudentEnrollmentSource(ctx, studentID, requestChildID, groupIDs)
	return err
}

// ReconcileTemplateRosters hands the pre-write rows to the Timetable owner's
// roster decision; a nil prior stays nil (coverage from scratch).
func (r bookingRosters) ReconcileTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from calendar.Date, prior []careplanCompose.RosterEnrollment) error {
	var rows []timetable.RosterEnrollment
	if prior != nil {
		rows = make([]timetable.RosterEnrollment, 0, len(prior))
		for _, row := range prior {
			rows = append(rows, timetable.RosterEnrollment{
				StudentID: row.StudentID, ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil,
				CalendarPeriodID: row.CalendarPeriodID, Weekday: row.Weekday,
				SelectedWeekdays: row.SelectedWeekdays, StudentAlumnus: row.StudentAlumnus,
			})
		}
	}
	_, _, err := r.rosters.ReconcileSourcedTemplateRosters(ctx, templateID, studentIDs, from, rows)
	return err
}

// bookingTemplates reads the live offering-sourced templates in id order and
// rewrites a template's source rule through the Timetable owner.
type bookingTemplates struct{ timetable bookingTimetableOwner }

func (t bookingTemplates) TemplatesWithOfferingSource(ctx context.Context) ([]careplanCompose.SourcedTemplate, error) {
	isTemplate := true
	return t.list(ctx, timetable.GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, HasOfferingSource: true, OrderByID: true})
}

func (t bookingTemplates) TemplatesSourcedFrom(ctx context.Context, offeringIDs []int64) ([]careplanCompose.SourcedTemplate, error) {
	if len(offeringIDs) == 0 {
		return []careplanCompose.SourcedTemplate{}, nil
	}
	isTemplate := true
	return t.list(ctx, timetable.GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, SourceOfferingIDs: offeringIDs, OrderByID: true})
}

func (t bookingTemplates) Groups(ctx context.Context, ids []int64) ([]careplanCompose.SourcedTemplate, error) {
	if len(ids) == 0 {
		return []careplanCompose.SourcedTemplate{}, nil
	}
	return t.list(ctx, timetable.GroupFilter{IDs: ids})
}

func (t bookingTemplates) list(ctx context.Context, filter timetable.GroupFilter) ([]careplanCompose.SourcedTemplate, error) {
	groups, err := t.timetable.ListGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.SourcedTemplate, 0, len(groups))
	for _, group := range groups {
		result = append(result, careplanCompose.SourcedTemplate{
			ID: group.ID, Name: group.Name, SeriesRootID: group.SeriesRootID, CalendarPeriodID: group.CalendarPeriodID,
			SourceCareOfferingIDs: group.SourceCareOfferingIDs, SourceGradeLevels: group.SourceGradeLevels,
			SourceSchoolClasses: group.SourceSchoolClasses,
		})
	}
	return result, nil
}

func (t bookingTemplates) UpdateTemplateOfferingSource(ctx context.Context, templateID int64, offeringIDs []int64, gradeLevels []int, schoolClasses []string) error {
	return t.timetable.UpdateGroupOfferingSource(ctx, templateID, timetable.OfferingSourceInput{
		CareOfferingIDs: offeringIDs, GradeLevels: gradeLevels, SchoolClasses: schoolClasses,
	})
}

type bookingPeriodReads interface {
	ListCalendarPeriods(context.Context, schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
}

// bookingPeriods lists the tenant's planning periods.
type bookingPeriods struct{ calendar bookingPeriodReads }

func (p bookingPeriods) Periods(ctx context.Context) ([]careplan.LinkedPeriod, error) {
	periods, err := p.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		return nil, err
	}
	result := make([]careplan.LinkedPeriod, 0, len(periods))
	for _, period := range periods {
		result = append(result, careplan.LinkedPeriod{
			ID: period.ID, StartDate: calendar.Date(period.StartDate), EndDate: calendar.Date(period.EndDate),
			IsActive: period.IsActive, WeekCycleLength: period.WeekCycleLength, WeekCycleAnchor: period.WeekCycleAnchor,
		})
	}
	return result, nil
}

type bookingEnrollmentReads interface {
	RequestByID(context.Context, int64, bool) (*enrollmentOwner.Request, error)
	ChildByID(context.Context, int64) (*enrollmentOwner.RequestChild, error)
	Phase(context.Context, int64) (*enrollmentOwner.Phase, error)
	Phases(context.Context) ([]*enrollmentOwner.Phase, error)
	PhasesByID(context.Context, []int64) ([]*enrollmentOwner.Phase, error)
	RequestChildOfferingsAtDate(context.Context, int64, enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	RequestChildOfferingHistory(context.Context, int64) ([]*enrollmentOwner.RequestChildOffering, error)
}

// bookingApprovedChildren resolves approved offering selections to their
// still-enrolled children.
type bookingApprovedChildren interface {
	ListApprovedChildrenByCareOfferingIDs(context.Context, []int64, calendar.Date) ([]*enrollmentOwner.ApprovedOfferingChild, error)
}

// bookingEnrollment reads Enrollment's requests, children, phases and booked
// selections, and the approved bookings with their students' classes.
type bookingEnrollment struct {
	owner    bookingEnrollmentReads
	approved bookingApprovedChildren
}

func (e bookingEnrollment) Request(ctx context.Context, id int64) (careplanCompose.BookingRequest, error) {
	request, err := e.owner.RequestByID(ctx, id, false)
	if err != nil {
		return careplanCompose.BookingRequest{}, err
	}
	if request == nil {
		return careplanCompose.BookingRequest{}, careplanCompose.BookingRowNotFound(fmt.Errorf("request %d not found", id))
	}
	return careplanCompose.BookingRequest{ID: request.ID, PhaseID: request.PhaseID}, nil
}

func (e bookingEnrollment) Child(ctx context.Context, id int64) (careplanCompose.BookingChild, error) {
	child, err := e.owner.ChildByID(ctx, id)
	if err != nil {
		return careplanCompose.BookingChild{}, err
	}
	if child == nil {
		return careplanCompose.BookingChild{}, careplanCompose.BookingRowNotFound(fmt.Errorf("request child %d not found", id))
	}
	return careplanCompose.BookingChild{
		ID: child.ID, RequestID: child.RequestID, Approved: child.Status == enrollmentOwner.ChildStatusApproved,
		CreatedStudentID: child.CreatedStudentID, TargetGradeLevel: child.TargetGradeLevel,
	}, nil
}

func (e bookingEnrollment) Phase(ctx context.Context, id int64) (careplanCompose.BookingPhase, error) {
	phase, err := e.owner.Phase(ctx, id)
	if err != nil {
		// The owner reports a missing row as a plain error, which the
		// materialization has always treated as a failure.
		return careplanCompose.BookingPhase{}, err
	}
	if phase == nil {
		return careplanCompose.BookingPhase{}, careplanCompose.BookingRowNotFound(fmt.Errorf("phase %d not found", id))
	}
	return careplanCompose.BookingPhase{OfferingPhase: bookingOfferingPhase(phase), CareOfferingSelectionMode: phase.CareOfferingSelectionMode}, nil
}

func (e bookingEnrollment) Phases(ctx context.Context) ([]careplan.OfferingPhase, error) {
	phases, err := e.owner.Phases(ctx)
	return bookingOfferingPhases(phases), err
}

func (e bookingEnrollment) PhasesByID(ctx context.Context, ids []int64) ([]careplan.OfferingPhase, error) {
	phases, err := e.owner.PhasesByID(ctx, ids)
	return bookingOfferingPhases(phases), err
}

func bookingOfferingPhases(phases []*enrollmentOwner.Phase) []careplan.OfferingPhase {
	result := make([]careplan.OfferingPhase, 0, len(phases))
	for _, phase := range phases {
		if phase != nil {
			result = append(result, bookingOfferingPhase(phase))
		}
	}
	return result
}

func bookingOfferingPhase(phase *enrollmentOwner.Phase) careplan.OfferingPhase {
	return careplan.OfferingPhase{
		ID: phase.ID, Name: phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate), ServiceEnd: calendar.Date(phase.ServiceEndDate),
	}
}

func (e bookingEnrollment) SelectionsAt(ctx context.Context, childID int64, on calendar.Date) ([]careplan.BookedOffering, error) {
	values, err := e.owner.RequestChildOfferingsAtDate(ctx, childID, enrollmentOwner.Date(on))
	return bookedOfferings(values), err
}

func (e bookingEnrollment) SelectionHistory(ctx context.Context, childID int64) ([]careplan.BookedOffering, error) {
	values, err := e.owner.RequestChildOfferingHistory(ctx, childID)
	return bookedOfferings(values), err
}

func bookedOfferings(values []*enrollmentOwner.RequestChildOffering) []careplan.BookedOffering {
	if values == nil {
		return nil
	}
	links := make([]careplan.BookedOffering, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		link := careplan.BookedOffering{
			RequestChildID: value.RequestChildID, CareOfferingID: value.CareOfferingID,
			SelectedDays: value.SelectedDays, ManualSelectedDays: value.ManualSelectedDays,
			AutomaticSelectedDays: value.AutomaticSelectedDays,
		}
		if value.ValidFrom != nil {
			link.ValidFrom = new(calendar.Date(*value.ValidFrom))
		}
		if value.ValidUntil != nil {
			link.ValidUntil = new(calendar.Date(*value.ValidUntil))
		}
		links = append(links, link)
	}
	return links
}

func (e bookingEnrollment) ApprovedChildren(ctx context.Context, offeringIDs []int64, onOrAfter calendar.Date) ([]careplanCompose.ApprovedOfferingChild, error) {
	children, err := e.approved.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, onOrAfter)
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.ApprovedOfferingChild, 0, len(children))
	for _, child := range children {
		if child == nil || child.Link == nil {
			continue
		}
		link := child.Link
		result = append(result, careplanCompose.ApprovedOfferingChild{
			Link: careplan.BookedOffering{
				RequestChildID: link.RequestChildID, CareOfferingID: link.CareOfferingID,
				SelectedDays: link.SelectedDays, ManualSelectedDays: link.ManualSelectedDays,
				AutomaticSelectedDays: link.AutomaticSelectedDays, ValidFrom: link.ValidFrom, ValidUntil: link.ValidUntil,
			},
			StudentID: child.StudentID, SchoolClass: child.SchoolClass, GradeLevel: enrollmentOwner.SchoolClassGradeLevel(child.SchoolClass),
		})
	}
	return result, nil
}

// bookingStudents reads students and takes the People Directory locks in the
// order every care writer shares: the class-writes gate, then the student row.
type bookingStudents struct{ people bookingStudentOwner }

func (s bookingStudents) Student(ctx context.Context, id int64, lock bool) (careplanCompose.BookingStudent, error) {
	mode := ""
	if lock {
		mode = "update"
	}
	row, err := s.people.ReadEnrollmentStudent(ctx, id, mode)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return careplanCompose.BookingStudent{}, careplanCompose.BookingRowNotFound(fmt.Errorf("%w: %w", sql.ErrNoRows, err))
	}
	if err != nil {
		return careplanCompose.BookingStudent{}, err
	}
	student := careplanCompose.BookingStudent{ID: row.ID, GradeLevel: enrollmentOwner.SchoolClassGradeLevel(row.SchoolClass)}
	// Both enrollment dates are validated, as the decision's student read
	// always did; only the care end bounds the rosters.
	if row.EnrolledFrom != "" {
		if _, err := calendar.ParseDate(row.EnrolledFrom); err != nil {
			return careplanCompose.BookingStudent{}, fmt.Errorf("decision: invalid owner enrollment date: %w", err)
		}
	}
	if row.EnrolledUntil != "" {
		until, err := calendar.ParseDate(row.EnrolledUntil)
		if err != nil {
			return careplanCompose.BookingStudent{}, fmt.Errorf("decision: invalid owner enrollment date: %w", err)
		}
		student.EnrolledUntil = &until
	}
	return student, nil
}

func (s bookingStudents) LockClassWrites(ctx context.Context) error {
	return s.people.LockEnrollmentClassWrites(ctx)
}

// LockStudents takes the students' care locks in the caller's transaction;
// missing students (concurrent offboarding) are skipped.
func (s bookingStudents) LockStudents(ctx context.Context, studentIDs []int64) error {
	for _, studentID := range studentIDs {
		if err := s.people.LockStudent(ctx, studentID); err != nil {
			if errors.Is(err, peopledirectory.ErrStudentNotFound) {
				continue
			}
			return err
		}
	}
	return nil
}

type bookingSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// bookingSettings resolves the tenant settings; without a settings service
// the registry defaults apply.
type bookingSettings struct{ settings bookingSettingsReads }

func (s bookingSettings) CareOfferingsEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return true, nil
	}
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCareOfferingsEnabled)
}

func (s bookingSettings) BookingsAuthoritative(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
}

type bookingActorPersons interface {
	FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error)
}

type bookingActorAccounts interface {
	FindAccount(ctx context.Context, id int64) (identityaccess.Account, error)
}

// bookingAdjustmentAudit appends the adjustment row to the Audit Platform's
// trail and names the actor from the People Directory and Identity & Access.
type bookingAdjustmentAudit struct {
	repo     auditModels.EnrollmentOfferingAdjustmentRepository
	persons  bookingActorPersons
	accounts bookingActorAccounts
}

func (a bookingAdjustmentAudit) RecordAdjustment(ctx context.Context, record careplan.OfferingAdjustmentRecord) (int64, error) {
	entry := &auditModels.EnrollmentOfferingAdjustment{
		RequestID: record.RequestID, RequestChildID: record.RequestChildID, StudentID: record.StudentID,
		ActorAccountID: record.ActorAccountID, ActorRole: record.ActorRole,
		ActorNameSnapshot: record.ActorNameSnapshot, ActorEmailSnapshot: record.ActorEmailSnapshot,
		Reason: record.Reason, Source: record.Source, Before: record.Before, After: record.After,
		CompleteWithdrawalConfirmed: record.CompleteWithdrawalConfirmed,
	}
	if err := a.repo.Create(ctx, entry); err != nil {
		return 0, err
	}
	return entry.ID, nil
}

func (a bookingAdjustmentAudit) ActorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	var name *string
	if a.persons != nil {
		if person, err := a.persons.FindByAccountID(ctx, accountID); err == nil && person != nil {
			fullName := strings.TrimSpace(person.GetFullName())
			if fullName != "" {
				name = &fullName
			}
		}
	}
	var email *string
	if a.accounts != nil {
		if account, err := a.accounts.FindAccount(ctx, accountID); err == nil && strings.TrimSpace(account.Email) != "" {
			value := account.Email
			email = &value
		}
	}
	return name, email
}

type bookingPickupReads interface {
	OfferingPickupForDate(ctx context.Context, studentID int64, date calendar.Date) (*careplan.PickupSchedule, error)
}

type bookingPickupRecords interface {
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	DeletePickupSchedule(context.Context, int64) error
}

// bookingPickup reads the offering pickup projection and Care Plan's stored
// weekly pickup rows it overrides.
type bookingPickup struct {
	baselines bookingPickupReads
	rows      bookingPickupRecords
}

func (p bookingPickup) OfferingPickupForDate(ctx context.Context, studentID int64, date calendar.Date) (*careplan.PickupSchedule, error) {
	return p.baselines.OfferingPickupForDate(ctx, studentID, date)
}

func (p bookingPickup) WeekdayRows(ctx context.Context, studentID int64) ([]careplanCompose.PickupWeekdayRow, error) {
	rows, err := p.rows.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}})
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.PickupWeekdayRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, careplanCompose.PickupWeekdayRow{ID: row.ID, Weekday: row.Weekday})
	}
	return result, nil
}

func (p bookingPickup) DeleteWeekdayRow(ctx context.Context, id int64) error {
	return p.rows.DeletePickupSchedule(ctx, id)
}

type bookingGuardianNotifier interface {
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}

// announceOfferingPickupChange announces a changed offering pickup
// projection once Care Plan's transaction committed: staff views refetch the
// pickup schedule, and the guardians' child cards refresh. Nil without a
// broadcaster and a notifier.
func announceOfferingPickupChange(broadcaster realtime.Broadcaster, notifier bookingGuardianNotifier, logger *slog.Logger) func(int64, []int64) {
	if broadcaster == nil && notifier == nil {
		return nil
	}
	return func(tenantID int64, studentIDs []int64) {
		if len(studentIDs) == 0 {
			return
		}
		if broadcaster != nil {
			source := "offering_pickup_projection"
			event := realtime.NewEvent(realtime.EventPickupScheduleChanged, "", realtime.EventData{Source: &source})
			if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
				logger.Warn(
					"offering pickup projection: failed to broadcast schedule change",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
			}
		}
		if notifier != nil {
			for _, studentID := range studentIDs {
				notifier.BroadcastChildUpdateToGuardians(tenantID, studentID)
			}
		}
	}
}
