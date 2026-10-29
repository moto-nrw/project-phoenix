package owners

import (
	"context"
	"errors"
	"fmt"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookingMaterializationOwners binds the materialization's ports over the
// owners it reads and writes. The caller completes the dependencies with the
// catalog, the settings, Care Plan's bookings and withdrawals, the effects and
// the clock, and composes them through newBookingMaterialization.
func BookingMaterializationOwners(
	timetables bookingTimetableOwner, rosters bookingRosterMaintenance, students bookingStudentOwner,
	periods bookingPeriodReads, enrollment bookingEnrollmentReads, approved bookingApprovedChildren,
	adjustments bookingAdjustmentTrail, persons bookingActorPersons, accounts bookingActorAccounts,
	pickup bookingPickupReads, pickupRows bookingPickupRecords, studentDays timetable.StudentTemplateDays,
) (careplanCompose.BookingMaterializationDependencies, error) {
	if timetables == nil || rosters == nil || students == nil || enrollment == nil ||
		approved == nil || adjustments == nil || pickup == nil || pickupRows == nil || studentDays == nil {
		return careplanCompose.BookingMaterializationDependencies{}, errors.New("booking materialization: timetable, rosters, students, enrollment, approved bookings, adjustment audit, pickup and student template days are required")
	}
	return careplanCompose.BookingMaterializationDependencies{
		Rosters:     bookingRosters{timetable: timetables, students: students, rosters: rosters},
		Templates:   bookingTemplates{timetable: timetables},
		Periods:     bookingPeriods{calendar: periods},
		Enrollment:  bookingEnrollment{owner: enrollment, approved: approved},
		Students:    bookingStudents{people: students},
		Audit:       bookingAdjustmentAudit{repo: adjustments, persons: persons, accounts: accounts},
		Pickup:      bookingPickup{baselines: pickup, rows: pickupRows},
		ManualPlans: bookingManualPlans{days: studentDays},
	}, nil
}

// bookingManualPlans asks the Timetable owner where a student stays planned
// by hand after a booking change (#5102).
type bookingManualPlans struct{ days timetable.StudentTemplateDays }

func (p bookingManualPlans) ManualStudentPlan(
	ctx context.Context, studentID int64, from calendar.Date, change careplan.BookingDayChange,
) ([]careplan.ManualRosterTemplate, error) {
	bookings := make([]timetable.StudentDayBooking, 0, len(change.Remaining))
	for _, booking := range change.Remaining {
		bookings = append(bookings, timetable.StudentDayBooking{
			Weekday: booking.Weekday, CountsAsCare: booking.CountsAsCare, PickupTime: booking.PickupTime,
		})
	}
	plans, err := p.days.ManualStudentPlan(ctx, timetable.ManualStudentPlanQuery{
		StudentID: studentID, From: from, Weekdays: change.DroppedWeekdays,
		NetLossWeekdays: change.NetLossWeekdays, Bookings: bookings,
	})
	if err != nil {
		return nil, err
	}
	result := make([]careplan.ManualRosterTemplate, 0, len(plans))
	for _, plan := range plans {
		result = append(result, careplan.ManualRosterTemplate{
			TemplateID: plan.TemplateID, Name: plan.Name, Weekdays: plan.Weekdays, ViaTarget: plan.ViaTarget,
		})
	}
	return result, nil
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
	return t.list(ctx, timetable.ActiveOfferingSourcedTemplates())
}

func (t bookingTemplates) TemplatesSourcedFrom(ctx context.Context, offeringIDs []int64) ([]careplanCompose.SourcedTemplate, error) {
	if len(offeringIDs) == 0 {
		return []careplanCompose.SourcedTemplate{}, nil
	}
	return t.list(ctx, timetable.ActiveTemplatesSourcedFrom(offeringIDs))
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
		ID: child.ID, RequestID: child.RequestID, Approved: child.IsApproved(),
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
	return careplanCompose.BookingPhase{OfferingPhase: BookingOfferingPhase(phase), CareOfferingSelectionMode: phase.CareOfferingSelectionMode}, nil
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
			result = append(result, BookingOfferingPhase(phase))
		}
	}
	return result
}

// BookingOfferingPhase is an Enrollment phase as Care Plan's booking flows
// read it; the root's offering-change review reads it too.
func BookingOfferingPhase(phase *enrollmentOwner.Phase) careplan.OfferingPhase {
	return careplan.OfferingPhase{
		ID: phase.ID, Name: phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate), ServiceEnd: calendar.Date(phase.ServiceEndDate),
	}
}

func (e bookingEnrollment) SelectionsAt(ctx context.Context, childID int64, on calendar.Date) ([]careplan.BookedOffering, error) {
	values, err := e.owner.RequestChildOfferingsAtDate(ctx, childID, enrollmentOwner.Date(on))
	return BookedOfferings(values), err
}

func (e bookingEnrollment) SelectionHistory(ctx context.Context, childID int64) ([]careplan.BookedOffering, error) {
	values, err := e.owner.RequestChildOfferingHistory(ctx, childID)
	return BookedOfferings(values), err
}

// BookedOfferings are Enrollment's selections as Care Plan's booked
// offerings; the root's offering-change review reads them too.
func BookedOfferings(values []*enrollmentOwner.RequestChildOffering) []careplan.BookedOffering {
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
		return careplanCompose.BookingStudent{}, careplanCompose.BookingRowNotFound(fmt.Errorf("%w: %w", transactionruntime.ErrNoRows, err))
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

type bookingActorPersons interface {
	FindByAccountID(ctx context.Context, accountID int64) (*peopledirectory.DirectoryPersonSnapshot, error)
}

type bookingActorAccounts interface {
	FindAccount(ctx context.Context, id int64) (identityaccess.Account, error)
}

// bookingAdjustmentTrail is the Audit Platform trail of the dated offering
// adjustments.
type bookingAdjustmentTrail interface {
	Create(ctx context.Context, entry *auditModels.EnrollmentOfferingAdjustment) error
}

// bookingAdjustmentAudit appends the adjustment row to the Audit Platform's
// trail and names the actor from the People Directory and Identity & Access.
type bookingAdjustmentAudit struct {
	repo     bookingAdjustmentTrail
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

// PickupScheduleBroadcaster wakes the staff pickup schedule views through the
// realtime hub.
type PickupScheduleBroadcaster struct{ Broadcaster realtime.Broadcaster }

func (b PickupScheduleBroadcaster) PickupScheduleChanged(tenantID int64) error {
	source := "offering_pickup_projection"
	return b.Broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventPickupScheduleChanged, "", realtime.EventData{Source: &source}))
}
