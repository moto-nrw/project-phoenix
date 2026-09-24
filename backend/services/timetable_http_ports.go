package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The ports of the timetable routes (modules/timetable/http) the root binds
// over retained services (#2732). The routes own the port shapes; these
// adapters translate the retained vocabulary into them and change nothing
// about which reads run.

// errAccountWithoutPerson reports an account that has no person row.
var errAccountWithoutPerson = errors.New("account has no person")

// TimetablePeople serves the People port of the timetable routes from the
// People Directory person service: display names, and whether a child still
// attends.
type TimetablePeople struct {
	persons users.PersonService
}

// NewTimetablePeople binds the People port to the person service.
func NewTimetablePeople(persons users.PersonService) TimetablePeople {
	return TimetablePeople{persons: persons}
}

// PersonNames maps person ids to "Vorname Nachname"; unknown persons are
// absent.
func (p TimetablePeople) PersonNames(ctx context.Context, personIDs []int64) (map[int64]string, error) {
	persons, err := p.persons.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(persons))
	for id, person := range persons {
		if person != nil {
			names[id] = person.GetFullName()
		}
	}
	return names, nil
}

// AccountPersonID returns the id of the account's person.
func (p TimetablePeople) AccountPersonID(ctx context.Context, accountID int64) (int64, error) {
	person, err := p.persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if person == nil {
		return 0, fmt.Errorf("account %d: %w", accountID, errAccountWithoutPerson)
	}
	return person.ID, nil
}

// AccountPersonName returns the display name of the account's person.
func (p TimetablePeople) AccountPersonName(ctx context.Context, accountID int64) (string, error) {
	person, err := p.persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return "", err
	}
	if person == nil {
		return "", fmt.Errorf("account %d: %w", accountID, errAccountWithoutPerson)
	}
	return person.GetFullName(), nil
}

// StaffPersonID returns the person id of a staff member, 0 when unknown.
func (p TimetablePeople) StaffPersonID(ctx context.Context, staffID int64) (int64, error) {
	staff, err := p.persons.GetStaffByID(ctx, staffID)
	if err != nil {
		return 0, err
	}
	if staff == nil || staff.ID == 0 {
		return 0, nil
	}
	return staff.PersonID, nil
}

// PersonStaffID returns the staff id of a person, 0 when the person is no
// staff member.
func (p TimetablePeople) PersonStaffID(ctx context.Context, personID int64) (int64, error) {
	staff, err := p.persons.GetStaffByPersonID(ctx, personID)
	if err != nil {
		return 0, err
	}
	if staff == nil {
		return 0, nil
	}
	return staff.ID, nil
}

// StaffNames maps staff ids to their person's display name in one read.
func (p TimetablePeople) StaffNames(ctx context.Context, staffIDs []int64) (map[int64]string, error) {
	staffByID, err := p.persons.GetStaffWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(staffByID))
	for id, staff := range staffByID {
		if staff != nil && staff.Person != nil {
			names[id] = staff.Person.GetFullName()
		}
	}
	return names, nil
}

// AttendingStudentPersons maps the children who exist and still attend on
// day to their person ids. Graduates and children past their end of care
// drop out (#405, #2487).
func (p TimetablePeople) AttendingStudentPersons(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]int64, error) {
	students, err := p.persons.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	persons := make(map[int64]int64, len(students))
	for id, student := range students {
		if student == nil || student.IsAlumnus() || student.CareEndedOn(day) {
			continue
		}
		persons[id] = student.PersonID
	}
	return persons, nil
}

// StudentAttends reports whether the child exists and still attends on day.
// The person service's not-found error passes through unchanged.
func (p TimetablePeople) StudentAttends(ctx context.Context, studentID int64, day calendar.Date) (bool, error) {
	student, err := p.persons.GetStudentByID(ctx, studentID)
	if err != nil {
		return false, err
	}
	if student == nil || student.ID == 0 || student.IsAlumnus() || student.CareEndedOn(day) {
		return false, nil
	}
	return true, nil
}

// TimetableSupervisionSheets serves the per-child sheet of the school portal
// (#2527) from the enrollment report, which audits every read.
type TimetableSupervisionSheets struct {
	report enrollment.ReportService
}

// NewTimetableSupervisionSheets binds the sheet port to the report.
func NewTimetableSupervisionSheets(report enrollment.ReportService) TimetableSupervisionSheets {
	return TimetableSupervisionSheets{report: report}
}

// SupervisionStudentSheet builds the sheet. A request the report refuses as
// an invalid filter carries SupervisionSheetRefused().
func (s TimetableSupervisionSheets) SupervisionStudentSheet(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
	companionBoundary []int64,
	actorAccountID int64,
	actorRole string,
) (any, error) {
	sheet, err := s.report.SupervisionStudentSheet(ctx, enrollment.SupervisionSheetInput{
		StudentID:         studentID,
		Date:              date,
		CompanionBoundary: companionBoundary,
		ActorAccountID:    actorAccountID,
		ActorRole:         actorRole,
	})
	if err != nil {
		if errors.Is(err, enrollment.ErrReportInvalidFilter) {
			return nil, supervisionSheetRefusal{err: err}
		}
		return nil, err
	}
	return sheet, nil
}

// supervisionSheetRefusal keeps the report's text and marks the refusal for
// the timetable routes.
type supervisionSheetRefusal struct {
	err error
}

func (r supervisionSheetRefusal) Error() string { return r.err.Error() }

func (r supervisionSheetRefusal) Unwrap() error { return r.err }

// SupervisionSheetRefused marks the refusal.
func (supervisionSheetRefusal) SupervisionSheetRefused() {}

// timetableOfferingSources serves the offering-source support of the
// Regeltermin editor from Care Plan's booking materialization (#2137, #3140,
// #3560).
type timetableOfferingSources struct {
	lister careplan.OfferingSourceEditor
}

// NewTimetableOfferingSources binds the offering-source support to Care
// Plan, or returns nil without it.
func NewTimetableOfferingSources(editor careplan.OfferingSourceEditor) timetable.OfferingSourceSupport {
	if editor == nil {
		return nil
	}
	return timetableOfferingSources{lister: editor}
}

func (s timetableOfferingSources) ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]timetable.OfferingSourceOption, error) {
	options, err := s.lister.ListOfferingSourceOptions(ctx, calendarPeriodID)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.OfferingSourceOption, 0, len(options))
	for _, option := range options {
		result = append(result, timetableOfferingSourceOption(option))
	}
	return result, nil
}

func timetableOfferingSourceOption(option careplan.OfferingSourceOption) timetable.OfferingSourceOption {
	sourced := make([]timetable.OfferingSourcedTemplate, 0, len(option.SourcedTemplates))
	for _, template := range option.SourcedTemplates {
		sourced = append(sourced, timetable.OfferingSourcedTemplate{
			ID:            template.ID,
			Name:          template.Name,
			GradeLevels:   template.GradeLevels,
			SchoolClasses: template.SchoolClasses,
		})
	}
	return timetable.OfferingSourceOption{
		ID:                     option.ID,
		Name:                   option.Name,
		PhaseID:                option.PhaseID,
		PhaseName:              option.PhaseName,
		PhaseServiceStart:      option.PhaseServiceStart,
		TotalCount:             option.TotalCount,
		GradeCounts:            option.GradeCounts,
		SourcedTemplates:       sourced,
		LegacyLinkedTemplateID: option.LegacyLinkedTemplateID,
	}
}

func (s timetableOfferingSources) CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (timetable.OfferingSourceCounts, error) {
	counts, err := s.lister.CombinedOfferingSourceCounts(ctx, offeringIDs, calendarPeriodID)
	if err != nil {
		return timetable.OfferingSourceCounts{}, err
	}
	students := make([]timetable.OfferingSourceStudent, 0, len(counts.Students))
	for _, student := range counts.Students {
		students = append(students, timetable.OfferingSourceStudent{
			StudentID:   student.StudentID,
			SchoolClass: student.SchoolClass,
		})
	}
	return timetable.OfferingSourceCounts{
		TotalCount:  counts.TotalCount,
		GradeCounts: counts.GradeCounts,
		Students:    students,
	}, nil
}

// EmptyRosterExplainer reads the offerings of the period once; the returned
// explainer classifies with the enrollment rule.
func (s timetableOfferingSources) EmptyRosterExplainer(ctx context.Context, calendarPeriodID *int64) (timetable.EmptyOfferingRosterExplainer, error) {
	options, err := s.lister.ListOfferingSourceOptions(ctx, calendarPeriodID)
	if err != nil {
		return nil, err
	}
	return func(selectedOfferingIDs []int64, date calendar.Date) *timetable.EmptyOfferingRoster {
		explanation := careplan.ExplainEmptyOfferingRoster(options, selectedOfferingIDs, date)
		if explanation == nil {
			return nil
		}
		return &timetable.EmptyOfferingRoster{
			Kind:             explanation.Kind,
			PhaseName:        explanation.PhaseName,
			ServiceStartDate: explanation.ServiceStartDate,
		}
	}, nil
}

// TemplateRosterMaintenance reads the offering feeds of all templates at
// once and derives each template's indicator; every queried template gets an
// entry.
func (s timetableOfferingSources) TemplateRosterMaintenance(ctx context.Context, templates []timetable.TemplateRosterMaintenanceQuery) (map[int64]timetable.TemplateRosterMaintenance, error) {
	queries := make([]careplan.TemplateRosterFeedQuery, 0, len(templates))
	for _, template := range templates {
		queries = append(queries, careplan.TemplateRosterFeedQuery{
			TemplateID:            template.TemplateID,
			CalendarPeriodID:      template.CalendarPeriodID,
			SourceCareOfferingIDs: template.SourceCareOfferingIDs,
		})
	}
	feeds, err := s.lister.TemplateRosterMaintenanceFeeds(ctx, queries)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]timetable.TemplateRosterMaintenance, len(templates))
	for _, template := range templates {
		derived := careplan.DeriveTemplateRosterMaintenance(careplan.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: template.SourceCareOfferingIDs,
			SourceGradeLevels:     template.SourceGradeLevels,
			SourceSchoolClasses:   template.SourceSchoolClasses,
			HasDynamicTargets:     template.HasDynamicTargets,
			Feeds:                 feeds[template.TemplateID],
		})
		result[template.TemplateID] = timetable.TemplateRosterMaintenance{
			Mode:                  string(derived.Mode),
			Offerings:             timetableOfferingRefs(derived.Offerings),
			GradeLevels:           derived.GradeLevels,
			SchoolClasses:         derived.SchoolClasses,
			InactiveOfferings:     timetableOfferingRefs(derived.InactiveOfferings),
			InvalidOfferings:      timetableOfferingRefs(derived.InvalidOfferings),
			DynamicTargetsManual:  derived.DynamicTargetsManual,
			CareOfferingsDisabled: derived.CareOfferingsDisabled,
		}
	}
	return result, nil
}

func timetableOfferingRefs(offerings []careplan.RosterMaintenanceOffering) []timetable.OfferingRef {
	result := make([]timetable.OfferingRef, 0, len(offerings))
	for _, offering := range offerings {
		result = append(result, timetable.OfferingRef{ID: offering.ID, Name: offering.Name})
	}
	return result
}
