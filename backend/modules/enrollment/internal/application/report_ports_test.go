package application

import (
	"context"
	"encoding/json"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Fakes of the report ports shared by the report tests.

// classRosterTestService wires the class roster with the fakes every roster
// test needs; tests swap single dependencies on the returned reports.
func classRosterTestService(students []*RosterStudent, persons map[int64]*RosterPerson, requests *fakeClassRosterRequestRepo, children *fakeClassRosterChildRepo) *Reports {
	return NewReports(ReportDependencies{
		Requests:          requests,
		Children:          children,
		Guardians:         &fakeClassRosterRequestGuardianRepo{},
		Offerings:         &fakeClassRosterCareOfferingRepo{},
		Phases:            &fakeClassRosterPhaseRepo{},
		Students:          &fakeClassRosterStudentRepo{students: students},
		GuardianContacts:  &fakeClassRosterStudentGuardianRepo{},
		Persons:           &fakeClassRosterPersonRepo{persons: persons},
		Groups:            &fakeEducationGroupRepo{},
		CareParticipation: allClassCareParticipation{},
	})
}

type fakeClassRosterStudentRepo struct {
	students  []*RosterStudent
	listCalls int
}

func (r *fakeClassRosterStudentRepo) ListClassRoster(_ context.Context, _ string) ([]*RosterStudent, error) {
	r.listCalls++
	return r.students, nil
}

type fakeClassRosterPersonRepo struct {
	persons map[int64]*RosterPerson
}

func (r *fakeClassRosterPersonRepo) PersonsByID(_ context.Context, ids []int64) (map[int64]*RosterPerson, error) {
	out := make(map[int64]*RosterPerson, len(ids))
	for _, id := range ids {
		if person := r.persons[id]; person != nil {
			out[id] = person
		}
	}
	return out, nil
}

type fakeEducationGroupRepo struct {
	groups  map[int64]string
	seenIDs []int64
}

func (r *fakeEducationGroupRepo) GroupNamesByID(_ context.Context, ids []int64) (map[int64]string, error) {
	r.seenIDs = append([]int64(nil), ids...)
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		if name, ok := r.groups[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

type fakeClassRosterRequestRepo struct {
	requests    []*enrollmentModels.Request
	seenFilters enrollment.RequestListFilters
}

// AdminRequests answers an unscoped read with more requests than an export
// may hold, so a roster that forgets to scope by student fails.
func (r *fakeClassRosterRequestRepo) AdminRequests(_ context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	r.seenFilters = filters
	if len(filters.CreatedStudentIDs) == 0 {
		return make([]*enrollment.Request, maxExportRequests+1), nil
	}
	return reportRequestInputs(r.requests)
}

type fakeCareUsageRequestRepo struct {
	requests []*enrollmentModels.Request
}

func (r *fakeCareUsageRequestRepo) AdminRequests(_ context.Context, _ enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	return reportRequestInputs(r.requests)
}

type fakeClassRosterChildRepo struct {
	children []*reportChild
}

func (r *fakeClassRosterChildRepo) ChildrenForRequests(_ context.Context, _ []int64) ([]*enrollment.RequestChild, error) {
	var values []*enrollment.RequestChild
	for _, child := range r.children {
		value, err := reportChildInput(child)
		if err != nil {
			return nil, err
		}
		// These report fixtures do not exercise birthdays, but owner rows
		// follow the database's required calendar-date contract.
		if value.DateOfBirth == "" {
			value.DateOfBirth = "2020-01-01"
		}
		values = append(values, value)
	}
	return values, nil
}

func (r *fakeClassRosterChildRepo) RequestChildOfferingsForChildrenAtDate(context.Context, []int64, enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	return nil, nil
}

type fakeClassRosterRequestGuardianRepo struct {
	guardians  []*enrollment.RequestGuardian
	err        error
	requestIDs []int64
}

func (r *fakeClassRosterRequestGuardianRepo) RequestGuardians(_ context.Context, requestIDs []int64) ([]*enrollment.RequestGuardian, error) {
	r.requestIDs = append([]int64(nil), requestIDs...)
	if r.err != nil {
		return nil, r.err
	}
	seen := map[int64]bool{}
	for _, id := range requestIDs {
		seen[id] = true
	}
	out := make([]*enrollment.RequestGuardian, 0, len(r.guardians))
	for _, guardian := range r.guardians {
		if guardian != nil && seen[guardian.RequestID] {
			copied := *guardian
			out = append(out, &copied)
		}
	}
	return out, nil
}

type fakeClassRosterCareOfferingRepo struct {
	offerings []*enrollmentModels.CareOffering
}

func (r *fakeClassRosterCareOfferingRepo) ListByPhase(_ context.Context, _ int64) ([]*enrollmentModels.CareOffering, error) {
	return r.offerings, nil
}

type classRosterSettingsStub struct {
	careOfferingsEnabled bool
	err                  error
}

func (s classRosterSettingsStub) CareOfferingsEnabled(context.Context) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.careOfferingsEnabled, nil
}

type fakeClassRosterStudentGuardianRepo struct {
	rows []GuardianContactRow
}

func (r *fakeClassRosterStudentGuardianRepo) GuardianContacts(_ context.Context, studentIDs []int64) ([]GuardianContactRow, error) {
	seen := map[int64]bool{}
	for _, id := range studentIDs {
		seen[id] = true
	}
	out := make([]GuardianContactRow, 0, len(r.rows))
	for _, row := range r.rows {
		if seen[row.StudentID] {
			out = append(out, row)
		}
	}
	return out, nil
}

type fakeClassRosterPhaseRepo struct {
	phase *enrollment.Phase
}

func (r *fakeClassRosterPhaseRepo) Phase(_ context.Context, id int64) (*enrollment.Phase, error) {
	if r.phase != nil {
		phase := *r.phase
		phase.ID = id
		return &phase, nil
	}
	return &enrollment.Phase{ID: id, Name: "Schuljahr 2026"}, nil
}

func (r *fakeClassRosterPhaseRepo) Phases(context.Context) ([]*enrollment.Phase, error) {
	if r.phase == nil {
		return nil, nil
	}
	phase := *r.phase
	return []*enrollment.Phase{&phase}, nil
}

// fakeClassDayPhaseRepo serves a fixed phase list for classDayPhases.
type fakeClassDayPhaseRepo struct {
	phases []*enrollment.Phase
}

func (r *fakeClassDayPhaseRepo) Phases(_ context.Context) ([]*enrollment.Phase, error) {
	var values []*enrollment.Phase
	for _, phase := range r.phases {
		if phase == nil {
			values = append(values, nil)
			continue
		}
		copied := *phase
		values = append(values, &copied)
	}
	return values, nil
}

func (r *fakeClassDayPhaseRepo) Phase(_ context.Context, id int64) (*enrollment.Phase, error) {
	for _, phase := range r.phases {
		if phase != nil && phase.ID == id {
			copied := *phase
			return &copied, nil
		}
	}
	return nil, enrollment.ErrReportPhaseNotFound
}

type fakeClassCareParticipation map[int64]bool

func (f fakeClassCareParticipation) ResolveListParticipation(
	_ context.Context, studentIDs []int64, _, _ calendar.Date, _ bool,
) (*careplan.CareParticipationResolution, error) {
	return &careplan.CareParticipationResolution{CandidateIDs: studentIDs, ParticipatingIDs: f}, nil
}

type allClassCareParticipation struct{}

func (allClassCareParticipation) ResolveListParticipation(
	_ context.Context, studentIDs []int64, _, _ calendar.Date, _ bool,
) (*careplan.CareParticipationResolution, error) {
	result := make(map[int64]bool, len(studentIDs))
	for _, studentID := range studentIDs {
		result[studentID] = true
	}
	return &careplan.CareParticipationResolution{CandidateIDs: studentIDs, ParticipatingIDs: result}, nil
}

type recordingClassCareParticipation struct {
	on    calendar.Date
	today calendar.Date
}

func (f *recordingClassCareParticipation) ResolveListParticipation(
	_ context.Context, studentIDs []int64, on, today calendar.Date, _ bool,
) (*careplan.CareParticipationResolution, error) {
	f.on, f.today = on, today
	return &careplan.CareParticipationResolution{
		CandidateIDs: studentIDs, ParticipatingIDs: map[int64]bool{studentIDs[0]: true},
	}, nil
}

type fakeCareUsagePickupScheduleSvc struct {
	rows       []*careplan.PickupSchedule
	err        error
	studentIDs []int64
	date       calendar.Date
}

func (s *fakeCareUsagePickupScheduleSvc) GetWeeklySchedulesByStudentIDsForDate(_ context.Context, studentIDs []int64, date calendar.Date) ([]*careplan.PickupSchedule, error) {
	s.studentIDs = append([]int64(nil), studentIDs...)
	s.date = date
	return s.rows, s.err
}

// fakeClassListEntries serves in-memory class-list entries (#2382) through
// the reader the report consumes.
type fakeClassListEntries struct {
	entries []ClassListEntry
}

func (r *fakeClassListEntries) ListClassListEntries(_ context.Context, schoolClass string) ([]ClassListEntry, error) {
	if schoolClass == "" {
		return r.entries, nil
	}
	key := strings.ToLower(strings.TrimSpace(schoolClass))
	var out []ClassListEntry
	for _, entry := range r.entries {
		if strings.ToLower(strings.TrimSpace(entry.SchoolClass)) == key {
			out = append(out, entry)
		}
	}
	return out, nil
}

func classListEntry(id int64, firstName, lastName, schoolClass string) ClassListEntry {
	return ClassListEntry{
		ID:          id,
		FirstName:   firstName,
		LastName:    lastName,
		SchoolClass: schoolClass,
	}
}

// fakeExportAccessLog records the phase export rows.
type fakeExportAccessLog struct {
	entries []ExportAccess
}

func (r *fakeExportAccessLog) RecordPhaseExport(_ context.Context, entry ExportAccess) error {
	r.entries = append(r.entries, entry)
	return nil
}

func (r *fakeExportAccessLog) RecordStudentExport(_ context.Context, _ int64, entry ExportAccess) error {
	r.entries = append(r.entries, entry)
	return nil
}

// reportRequestInputs hands request fixtures over the way the owner stores
// them: answers as raw JSON.
func reportRequestInputs(requests []*enrollmentModels.Request) ([]*enrollment.Request, error) {
	var values []*enrollment.Request
	for _, r := range requests {
		if r == nil {
			values = append(values, nil)
			continue
		}
		value := &enrollment.Request{
			ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			SchemaID: r.SchemaID, PhaseID: r.PhaseID,
			GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName,
			GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID,
			SubmissionSource: r.SubmissionSource, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires,
			SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode,
		}
		var err error
		if value.ConsentFlags, err = json.Marshal(r.ConsentFlags); err != nil {
			return nil, err
		}
		if value.LegalBlocksSnapshot, err = json.Marshal(r.LegalBlocksSnapshot); err != nil {
			return nil, err
		}
		if value.CustomData, err = json.Marshal(r.CustomData); err != nil {
			return nil, err
		}
		if value.SourceMetadata, err = json.Marshal(r.SourceMetadata); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// reportChildInput hands a child fixture over the way the owner stores it.
func reportChildInput(r *reportChild) (*enrollment.RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollment.RequestChild{
		ID: r.ID, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName,
		DateOfBirth:      enrollment.Date(r.DateOfBirth),
		TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, Status: r.Status,
		CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID,
	}
	data, err := json.Marshal(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.CustomData = data
	return result, nil
}
