package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// classRosterInputs are the reads one class roster combines for an already
// loaded student set.
type classRosterInputs struct {
	phase               *enrollment.Phase
	students            []*RosterStudent
	studentGuardians    map[int64][]enrollment.ClassRosterGuardian
	companions          map[int64][]departure.CompanionLink
	persons             map[int64]*RosterPerson
	groups              map[int64]string
	enrollments         map[int64]*classRosterApprovedEnrollment
	offeringByID        map[int64]*enrollmentModels.CareOffering
	schemas             map[int64]*enrollment.FormSchema
	careOfferingsActive bool
	schedulePickup      map[int64]map[string]string
}

// classRosterEnrollmentReads are the enrollment rows of the students' phase.
type classRosterEnrollmentReads struct {
	requests         []*enrollmentModels.Request
	children         []*reportChild
	requestGuardians map[int64][]*enrollment.RequestGuardian
}

func (s *Reports) ExportClassRoster(ctx context.Context, filters enrollment.ClassRosterFilters, actorAccountID int64, actorRole, format string) (*enrollment.ClassRosterReport, error) {
	report, err := s.classRoster(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordClassRosterExportAudit(ctx, report, actorAccountID, actorRole, format); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Reports) classRoster(ctx context.Context, filters enrollment.ClassRosterFilters) (*enrollment.ClassRosterReport, error) {
	filters.SchoolClass = strings.TrimSpace(filters.SchoolClass)
	if err := validateClassRosterFilters(filters); err != nil {
		return nil, err
	}
	if s.deps.Students == nil {
		return nil, fmt.Errorf("class roster report: student repo not configured")
	}
	students, err := s.classRosterStudents(ctx, filters)
	if err != nil {
		return nil, err
	}
	report, err := s.classRosterForStudents(ctx, filters, students)
	if err != nil {
		return nil, err
	}
	if err := s.appendClassListEntries(ctx, filters, report); err != nil {
		return nil, err
	}
	return report, nil
}

func validateClassRosterFilters(filters enrollment.ClassRosterFilters) error {
	if filters.PhaseID <= 0 {
		return fmt.Errorf("%w: phase_id is required", enrollment.ErrReportInvalidFilter)
	}
	if filters.AllClasses && filters.SchoolClass != "" {
		return fmt.Errorf("%w: school_class and all_classes are mutually exclusive", enrollment.ErrReportInvalidFilter)
	}
	if !filters.AllClasses && filters.SchoolClass == "" {
		return fmt.Errorf("%w: school_class is required", enrollment.ErrReportInvalidFilter)
	}
	return nil
}

// classRosterCareDate is the day a roster describes. It decides which
// children still belong to the class for care purposes (#2487): the class day
// view pages through the week and passes the day it renders, every other
// caller means today.
func classRosterCareDate(filters enrollment.ClassRosterFilters, today calendar.Date) calendar.Date {
	if filters.OfferingDate != nil {
		return *filters.OfferingDate
	}
	return today
}

// appendClassListEntries folds the class-list-only entries (#2382) into the
// roster: children of the Klassenverband without any OGS record, marked
// "Keine Betreuung". They sort into their class alphabetically like every
// student row; the AllClasses heading logic keys off the row's class, so a
// class that exists only through entries gets its own section automatically.
func (s *Reports) appendClassListEntries(ctx context.Context, filters enrollment.ClassRosterFilters, report *enrollment.ClassRosterReport) error {
	rows, err := s.classListEntryRows(ctx, filters.SchoolClass, filters.AllClasses)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	if len(report.Rows)+len(rows) > maxReportRows {
		return fmt.Errorf("class roster report: %d rows: %w", len(report.Rows)+len(rows), enrollment.ErrReportExportTooLarge)
	}
	report.Rows = append(report.Rows, rows...)
	sortClassRosterRows(report.Rows)
	report.Totals.Students += len(rows)
	report.Totals.ListEntries += len(rows)
	return nil
}

// classListEntryRows loads the class-list-only entries — one class or all —
// and renders them as roster rows. Without the binding there are none.
func (s *Reports) classListEntryRows(ctx context.Context, schoolClass string, allClasses bool) ([]enrollment.ClassRosterRow, error) {
	if s.deps.ClassListEntries == nil {
		return nil, nil
	}
	if allClasses {
		schoolClass = ""
	}
	entries, err := s.deps.ClassListEntries.ListClassListEntries(ctx, schoolClass)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list class list entries: %w", err)
	}
	rows := make([]enrollment.ClassRosterRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, enrollment.ClassRosterRow{
			ListEntry:         true,
			ListEntryID:       entry.ID,
			FirstName:         entry.FirstName,
			LastName:          entry.LastName,
			SchoolClass:       entry.SchoolClass,
			EnrollmentSummary: enrollment.ClassListEntryNoCareLabel,
			Offerings:         []enrollment.CareUsageRowOffering{},
			OfferingsByDay:    map[string][]string{},
			CareDays:          []string{},
			ArrivalByDay:      map[string]string{},
			PickupByDay:       map[string]string{},
			DepartureByDay:    map[string]string{},
			Guardians:         []enrollment.ClassRosterGuardian{},
		})
	}
	return rows, nil
}

// classRosterStudents loads one selected class or every student carrying a
// non-empty class name. The shared participation rule filters that candidate
// set after the one bulk query.
func (s *Reports) classRosterStudents(ctx context.Context, filters enrollment.ClassRosterFilters) ([]*RosterStudent, error) {
	schoolClass := ""
	if !filters.AllClasses {
		schoolClass = filters.SchoolClass
	}
	students, err := s.deps.Students.ListClassRoster(ctx, schoolClass)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list students: %w", err)
	}
	if !filters.AllClasses {
		students = studentsInClass(students, filters.SchoolClass)
	} else {
		students = studentsWithClass(students)
	}
	today := s.today()
	students, err = s.filterClassParticipation(ctx, students, classRosterCareDate(filters, today), today)
	if err != nil {
		return nil, err
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class roster report: %d students: %w", len(students), enrollment.ErrReportExportTooLarge)
	}
	return students, nil
}

func studentsInClass(students []*RosterStudent, schoolClass string) []*RosterStudent {
	result := students[:0]
	for _, student := range students {
		if student != nil && strings.EqualFold(strings.TrimSpace(student.SchoolClass), strings.TrimSpace(schoolClass)) {
			result = append(result, student)
		}
	}
	return result
}

func studentsWithClass(students []*RosterStudent) []*RosterStudent {
	result := students[:0]
	for _, student := range students {
		if student != nil && strings.TrimSpace(student.SchoolClass) != "" {
			result = append(result, student)
		}
	}
	return result
}

func (s *Reports) filterClassParticipation(ctx context.Context, students []*RosterStudent, day, today calendar.Date) ([]*RosterStudent, error) {
	if s.deps.CareParticipation == nil {
		return nil, errors.New("class roster report: care participation resolver is not configured")
	}
	if len(students) == 0 {
		return students, nil
	}
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	resolution, err := s.deps.CareParticipation.ResolveListParticipation(ctx, ids, day, today, false)
	if err != nil {
		return nil, fmt.Errorf("class roster report: apply care participation: %w", err)
	}
	kept := make([]*RosterStudent, 0, len(students))
	for _, student := range students {
		if resolution.ParticipatingIDs[student.ID] {
			kept = append(kept, student)
		}
	}
	return kept, nil
}

// classRosterForStudents builds the roster for an already-loaded student
// set. The class day view merges the roster of EVERY covering phase for every
// class on the teachers' landing page, so it loads the class once and reuses
// it across phases instead of re-querying identical students per phase.
// Callers pass normalized, validated filters.
func (s *Reports) classRosterForStudents(ctx context.Context, filters enrollment.ClassRosterFilters, students []*RosterStudent) (*enrollment.ClassRosterReport, error) {
	if err := s.classRosterConfigured(filters); err != nil {
		return nil, err
	}
	phase, err := s.deps.Phases.Phase(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("class roster report: phase %d: %w", filters.PhaseID, enrollment.ErrReportPhaseNotFound)
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class roster report: %d students: %w", len(students), enrollment.ErrReportExportTooLarge)
	}
	in, err := s.loadClassRosterInputs(ctx, filters, phase, students)
	if err != nil {
		return nil, err
	}
	rows := make([]enrollment.ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		row, err := in.row(student)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sortClassRosterRows(rows)
	report := &enrollment.ClassRosterReport{
		Phase: enrollment.CareUsagePhase{ID: phase.ID, Name: phase.Name},
		Filters: enrollment.ClassRosterAppliedFilters{
			PhaseID:     filters.PhaseID,
			SchoolClass: filters.SchoolClass,
			AllClasses:  filters.AllClasses,
			Status:      enrollment.ChildStatusApproved,
		},
		Totals: enrollment.ClassRosterTotals{Students: len(rows)},
		Rows:   rows,
	}
	for _, row := range rows {
		if row.Registered {
			report.Totals.Registered++
		}
	}
	return report, nil
}

func (s *Reports) classRosterConfigured(filters enrollment.ClassRosterFilters) error {
	if s.deps.Persons == nil {
		return fmt.Errorf("class roster report: person repo not configured")
	}
	if s.deps.Groups == nil {
		return fmt.Errorf("class roster report: education group repo not configured")
	}
	if filters.SkipGuardianData {
		return nil
	}
	if s.deps.GuardianContacts == nil {
		return fmt.Errorf("class roster report: student guardian repo not configured")
	}
	if s.deps.Guardians == nil {
		return fmt.Errorf("class roster report: request guardian repo not configured")
	}
	return nil
}

func (s *Reports) loadClassRosterInputs(ctx context.Context, filters enrollment.ClassRosterFilters, phase *enrollment.Phase, students []*RosterStudent) (*classRosterInputs, error) {
	in := &classRosterInputs{
		phase:            phase,
		students:         students,
		studentGuardians: map[int64][]enrollment.ClassRosterGuardian{},
		companions:       map[int64][]departure.CompanionLink{},
	}
	studentIDs := classRosterStudentIDs(students)
	if err := s.loadClassRosterStudentReads(ctx, filters, in, studentIDs); err != nil {
		return nil, err
	}
	reads, err := s.loadClassRosterEnrollments(ctx, filters, students, studentIDs)
	if err != nil {
		return nil, err
	}
	if err := s.loadClassRosterCatalog(ctx, filters, in, reads.requests); err != nil {
		return nil, err
	}
	offeringDate, err := s.attachClassRosterSelections(ctx, filters, in, reads)
	if err != nil {
		return nil, err
	}
	schedulePickup, err := s.schedulePickupByStudentIDs(ctx, classRosterAllStudentIDs(students), offeringDate)
	if err != nil {
		return nil, fmt.Errorf("class roster report: %w", err)
	}
	in.schedulePickup = schedulePickup
	return in, nil
}

// loadClassRosterStudentReads reads the students' guardian contacts and
// companion links (unless the caller skips guardian data), names and groups.
func (s *Reports) loadClassRosterStudentReads(ctx context.Context, filters enrollment.ClassRosterFilters, in *classRosterInputs, studentIDs []int64) error {
	var err error
	if !filters.SkipGuardianData {
		if in.studentGuardians, err = s.classRosterStudentGuardianContacts(ctx, studentIDs); err != nil {
			return err
		}
		if in.companions, err = s.classRosterCompanions(ctx, studentIDs); err != nil {
			return err
		}
	}
	if in.persons, err = s.deps.Persons.PersonsByID(ctx, classRosterPersonIDs(in.students)); err != nil {
		return fmt.Errorf("class roster report: load persons: %w", err)
	}
	if groupIDs := classRosterGroupIDs(in.students); len(groupIDs) > 0 {
		if in.groups, err = s.deps.Groups.GroupNamesByID(ctx, groupIDs); err != nil {
			return fmt.Errorf("class roster report: load groups: %w", err)
		}
	}
	return nil
}

// loadClassRosterEnrollments reads the phase's requests of the students,
// their children and, unless skipped, the request guardians.
func (s *Reports) loadClassRosterEnrollments(ctx context.Context, filters enrollment.ClassRosterFilters, students []*RosterStudent, studentIDs []int64) (*classRosterEnrollmentReads, error) {
	reads := &classRosterEnrollmentReads{requestGuardians: map[int64][]*enrollment.RequestGuardian{}}
	if len(studentIDs) == 0 {
		return reads, nil
	}
	var err error
	reads.requests, err = listReportRequests(ctx, s.deps.Requests, enrollment.RequestListFilters{
		PhaseID:           filters.PhaseID,
		CreatedStudentIDs: studentIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("class roster report: list requests: %w", err)
	}
	if len(reads.requests) > maxExportRequests {
		return nil, fmt.Errorf("class roster report: %d requests: %w", len(reads.requests), enrollment.ErrReportExportTooLarge)
	}
	requestIDs := classRosterRequestIDs(reads.requests)
	if reads.children, err = listReportChildren(ctx, s.deps.Children, requestIDs); err != nil {
		return nil, fmt.Errorf("class roster report: list children: %w", err)
	}
	if !filters.SkipGuardianData {
		guardians, err := s.deps.Guardians.RequestGuardians(ctx, requestIDs)
		if err != nil {
			return nil, fmt.Errorf("class roster report: list request guardians: %w", err)
		}
		reads.requestGuardians = requestGuardiansByRequestID(guardians)
	}
	reads.children = classRosterChildrenForStudents(reads.children, classRosterStudentsByID(students))
	if len(reads.children) > maxReportRows {
		return nil, fmt.Errorf("class roster report: %d children: %w", len(reads.children), enrollment.ErrReportExportTooLarge)
	}
	return reads, nil
}

// loadClassRosterCatalog reads the phase's offerings, the request schemas
// and whether the offering catalog constrains the care days.
func (s *Reports) loadClassRosterCatalog(ctx context.Context, filters enrollment.ClassRosterFilters, in *classRosterInputs, requests []*enrollmentModels.Request) error {
	offerings, err := s.deps.Offerings.ListByPhase(ctx, filters.PhaseID)
	if err != nil {
		return fmt.Errorf("class roster report: list offerings: %w", err)
	}
	in.offeringByID = make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			in.offeringByID[offering.ID] = offering
		}
	}
	if in.schemas, err = s.loadSchemas(ctx, requests); err != nil {
		return err
	}
	in.careOfferingsActive, err = s.careOfferingsEnabled(ctx, "class roster report")
	return err
}

// attachClassRosterSelections picks every student's newest approved
// enrollment and attaches its offering selections on the roster's offering
// date and its request guardians. It returns the offering date.
func (s *Reports) attachClassRosterSelections(ctx context.Context, filters enrollment.ClassRosterFilters, in *classRosterInputs, reads *classRosterEnrollmentReads) (calendar.Date, error) {
	enrollments, approvedChildIDs := classRosterApprovedEnrollments(reads.children, classRosterRequestsByID(reads.requests), classRosterStudentsByID(in.students))
	offeringDate := enrollment.ReportOfferingDate(s.today(), in.phase)
	if filters.OfferingDate != nil {
		offeringDate = *filters.OfferingDate
	}
	values, err := s.deps.Children.RequestChildOfferingsForChildrenAtDate(ctx, approvedChildIDs, enrollment.Date(offeringDate))
	links := enrollment.RequestChildOfferingRecordsOf(values)
	if err != nil {
		return "", fmt.Errorf("class roster report: list child offerings: %w", err)
	}
	classRosterAttachOfferingLinks(enrollments, links)
	classRosterAttachRequestGuardians(enrollments, reads.requestGuardians)
	in.enrollments = enrollments
	return offeringDate, nil
}

func sortClassRosterRows(rows []enrollment.ClassRosterRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if r := compareSchoolClasses(rows[i].SchoolClass, rows[j].SchoolClass); r != 0 {
			return r < 0
		}
		if r := compareGermanNames(rows[i].LastName, rows[i].FirstName, rows[j].LastName, rows[j].FirstName); r != 0 {
			return r < 0
		}
		if rows[i].StudentID != rows[j].StudentID {
			return rows[i].StudentID < rows[j].StudentID
		}
		return rows[i].ListEntryID < rows[j].ListEntryID
	})
}

// classRosterCompanions loads the "läuft mit" links of every listed child, so
// an accompanied departure names the children it means. Without the binding
// only the names in that one column are missing, never the report.
func (s *Reports) classRosterCompanions(ctx context.Context, studentIDs []int64) (map[int64][]departure.CompanionLink, error) {
	if s.deps.Companions == nil || len(studentIDs) == 0 {
		return map[int64][]departure.CompanionLink{}, nil
	}
	links, err := s.deps.Companions.ListLinksForStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: load departure companions: %w", err)
	}
	return links, nil
}

func (s *Reports) classRosterStudentGuardianContacts(ctx context.Context, studentIDs []int64) (map[int64][]enrollment.ClassRosterGuardian, error) {
	out := make(map[int64][]enrollment.ClassRosterGuardian, len(studentIDs))
	if len(studentIDs) == 0 {
		return out, nil
	}
	rows, err := s.deps.GuardianContacts.GuardianContacts(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: load student guardians: %w", err)
	}
	return classRosterStudentGuardianContactsFromRows(rows), nil
}
