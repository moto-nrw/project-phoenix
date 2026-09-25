package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// phaseResponseCountedStatuses are the child statuses that count as "the
// family answered". The question in #3379 is about handing the form in, not
// about the school's decision, so every submitted state counts whatever the
// school did with it afterwards.
//
// Deliberately absent: withdrawn (the family took the answer back) and all
// rollover-intermediate statuses. In particular, auto_renewed stays open
// until the deadline worker turns it into submitted or approved: the family
// can still decline an opt-out rollover.
var phaseResponseCountedStatuses = []string{
	enrollment.ChildStatusSubmitted,
	enrollment.ChildStatusUnderReview,
	enrollment.ChildStatusApproved,
	enrollment.ChildStatusWaitlisted,
	enrollment.ChildStatusRejected,
}

// phaseResponseWaitingStatuses are rollover rows that exist for a child but
// are no answer. The overview links them so the school finds the open row.
var phaseResponseWaitingStatuses = []string{
	enrollment.ChildStatusPendingRenewal,
	enrollment.ChildStatusAutoRenewed,
	enrollment.ChildStatusPendingAdminReview,
}

// phaseResponseApplicable says whether submissions of this phase carry a
// reference to the existing child. Only the existing_students audience pins
// matched_student_id at submit, and only a rollover copies a child forward.
// Every other phase stores name and birthday alone, and guessing the child
// from a name would put wrong families on a call list.
func phaseResponseApplicable(phase *enrollment.Phase) bool {
	return phase.Audience == enrollment.PhaseAudienceExistingStudents ||
		phase.RolloverSourcePhaseID != nil
}

// ResponseOverview compares the school's current children with the
// submissions of one phase (#3379): who answered, who is still missing.
func (s *Phases) ResponseOverview(ctx context.Context, phaseID int64) (*enrollment.PhaseResponseOverview, error) {
	if !s.deps.Responses.complete() {
		return nil, enrollment.ErrPhaseResponseOverviewUnavailable
	}
	phase, err := s.PhaseByID(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	if !phaseResponseApplicable(phase) {
		return &enrollment.PhaseResponseOverview{Rows: []enrollment.PhaseResponseRow{}, Excluded: []enrollment.PhaseResponseExclusion{}}, nil
	}
	answers, waiting, err := s.phaseResponseLinks(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	today := s.todayDate()
	roster, err := s.deps.Responses.Roster.ListRunningStudents(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("phase %d response overview: list roster: %w", phaseID, err)
	}
	roster = phaseResponseRosterWithAnswers(roster, answers)
	ids := make([]int64, 0, len(roster))
	for _, student := range roster {
		ids = append(ids, student.ID)
	}
	exits, err := s.deps.Responses.CareExits.StudentsWithCareExit(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("phase %d response overview: list care exits: %w", phaseID, err)
	}
	apps, err := s.deps.Responses.PortalAccounts.StudentsWithPortalGuardian(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("phase %d response overview: list portal accounts: %w", phaseID, err)
	}
	gradeMax, err := s.phaseResponseGradeMax(ctx)
	if err != nil {
		return nil, err
	}
	scope := newPhaseResponseScope(phase, today, gradeMax)
	return buildPhaseResponseOverview(roster, scope, exits, apps, answers, waiting), nil
}

func (s *Phases) phaseResponseGradeMax(ctx context.Context) (int, error) {
	if s.deps.Settings == nil {
		return defaultGradeLevelMax, nil
	}
	value, err := s.deps.Settings.GradeLevelMax(ctx)
	if err != nil {
		return 0, fmt.Errorf("phase response overview: resolve grade level max: %w", err)
	}
	return value, nil
}

// phaseResponseLink is the submission that stands for one existing child.
type phaseResponseLink struct {
	requestID   int64
	status      string
	firstName   string
	lastName    string
	schoolClass string
}

// phaseResponseLinks maps existing children to their counted answer and to
// their still-open rollover row. A child with several submissions counts
// once; the earliest submission is the one shown.
func (s *Phases) phaseResponseLinks(ctx context.Context, phaseID int64) (answers, waiting map[int64]phaseResponseLink, err error) {
	statuses := append(append([]string{}, phaseResponseCountedStatuses...), phaseResponseWaitingStatuses...)
	phaseChildren, err := s.deps.Records.PhaseResponseChildren(ctx, phaseID, statuses)
	if err != nil {
		return nil, nil, fmt.Errorf("phase %d response overview: list children: %w", phaseID, err)
	}
	children := make([]*enrollment.RequestChild, 0, len(phaseChildren))
	sources := make(map[int64]*enrollment.RequestChild)
	for _, phaseChild := range phaseChildren {
		if phaseChild.IsPhaseChild {
			children = append(children, phaseChild.Child)
			continue
		}
		sources[phaseChild.Child.ID] = phaseChild.Child
	}
	sort.SliceStable(children, func(i, j int) bool { return children[i].ID < children[j].ID })
	answers = make(map[int64]phaseResponseLink)
	waiting = make(map[int64]phaseResponseLink)
	for _, child := range children {
		studentID := phaseResponseStudentID(child, sources)
		if studentID == 0 {
			continue
		}
		target := waiting
		if phaseResponseCounts(child.Status) {
			target = answers
		}
		if _, seen := target[studentID]; !seen {
			target[studentID] = phaseResponseLink{
				requestID: child.RequestID, status: child.Status,
				firstName: child.FirstName, lastName: child.LastName,
				schoolClass: phaseResponseChildSchoolClass(child),
			}
		}
	}
	return answers, waiting, nil
}

func phaseResponseChildSchoolClass(child *enrollment.RequestChild) string {
	if child.TargetSchoolClass == nil {
		return ""
	}
	return *child.TargetSchoolClass
}

func phaseResponseCounts(status string) bool {
	for _, counted := range phaseResponseCountedStatuses {
		if status == counted {
			return true
		}
	}
	return false
}

func phaseResponsePinnedStudent(child *enrollment.RequestChild) int64 {
	if child.MatchedStudentID != nil {
		return *child.MatchedStudentID
	}
	if child.CreatedStudentID != nil {
		return *child.CreatedStudentID
	}
	return 0
}

// phaseResponseStudentID resolves the existing child a submission belongs
// to: its own pinned reference first, else the student of the row the
// rollover copied it from, walking the complete rollover chain.
func phaseResponseStudentID(child *enrollment.RequestChild, sources map[int64]*enrollment.RequestChild) int64 {
	seen := map[int64]struct{}{child.ID: {}}
	for child != nil {
		if id := phaseResponsePinnedStudent(child); id != 0 {
			return id
		}
		if child.RolloverSourceChildID == nil {
			return 0
		}
		sourceID := *child.RolloverSourceChildID
		if _, repeated := seen[sourceID]; repeated {
			return 0
		}
		seen[sourceID] = struct{}{}
		var ok bool
		child, ok = sources[sourceID]
		if !ok {
			return 0
		}
	}
	return 0
}

// phaseResponseScope decides which children of the roster a phase expects.
type phaseResponseScope struct {
	// advanceGrade is true when the phase expects the next school year's
	// grade. Half-year rollovers can keep the current grade explicitly.
	advanceGrade bool
	gradeMax     int
	grades       map[int]struct{}
	classes      map[string]struct{}
}

func newPhaseResponseScope(phase *enrollment.Phase, today calendar.Date, gradeMax int) phaseResponseScope {
	scope := phaseResponseScope{
		advanceGrade: phase.Kind == enrollment.PhaseKindSchoolYear &&
			string(phase.ServiceStartDate) > today.String() &&
			(phase.RolloverSourcePhaseID == nil || phase.RolloverBumpsGrade),
		gradeMax: gradeMax,
		grades:   map[int]struct{}{},
		classes:  map[string]struct{}{},
	}
	for _, grade := range phase.EligibleGradeLevels {
		scope.grades[grade] = struct{}{}
	}
	for _, class := range phase.EligibleSchoolClasses {
		if class = normalizeClass(class); class != "" {
			scope.classes[class] = struct{}{}
		}
	}
	if len(scope.grades) == 0 {
		// The eligible classes are what the family declares for the phase.
		// Their concrete names stay in classes; their grades are additionally
		// compared with today's class.
		for class := range scope.classes {
			if grade, ok := phaseResponseGrade(class); ok {
				scope.grades[grade] = struct{}{}
			}
		}
	}
	return scope
}

func phaseResponseGrade(class string) (int, bool) {
	grade, err := strconv.Atoi(gradePrefix(class))
	if err != nil {
		return 0, false
	}
	return grade, true
}

// exclusion returns why the phase does not expect this child, or "". A class
// without a grade number ("Bienen") is checked against a concrete class
// restriction without progression; otherwise it stays expected because asking
// one family too many is cheaper than silently dropping one.
func (s phaseResponseScope) exclusion(class string) string {
	class = normalizeClass(class)
	grade, ok := phaseResponseGrade(class)
	if s.advanceGrade && ok {
		if grade >= s.gradeMax {
			return enrollment.PhaseResponseExcludedGraduating
		}
		grade++
		class = phaseResponseClassWithGrade(class, grade)
	}
	if len(s.classes) > 0 {
		if _, eligible := s.classes[class]; !eligible {
			return enrollment.PhaseResponseExcludedNotInScope
		}
	}
	if !ok || len(s.grades) == 0 {
		return ""
	}
	if _, eligible := s.grades[grade]; !eligible {
		return enrollment.PhaseResponseExcludedNotInScope
	}
	return ""
}

func phaseResponseClassWithGrade(class string, grade int) string {
	class = normalizeClass(class)
	prefix := gradePrefix(class)
	index := strings.Index(class, prefix)
	if index < 0 {
		return class
	}
	return class[:index] + strconv.Itoa(grade) + class[index+len(prefix):]
}

func phaseResponseRosterWithAnswers(roster []enrollment.PhaseResponseStudent, answers map[int64]phaseResponseLink) []enrollment.PhaseResponseStudent {
	present := make(map[int64]struct{}, len(roster))
	for _, student := range roster {
		present[student.ID] = struct{}{}
	}
	for studentID, answer := range answers {
		if _, exists := present[studentID]; exists {
			continue
		}
		roster = append(roster, enrollment.PhaseResponseStudent{
			ID: studentID, FirstName: answer.firstName, LastName: answer.lastName, SchoolClass: answer.schoolClass,
		})
	}
	return roster
}

func buildPhaseResponseOverview(
	roster []enrollment.PhaseResponseStudent,
	scope phaseResponseScope,
	exits, apps map[int64]bool,
	answers, waiting map[int64]phaseResponseLink,
) *enrollment.PhaseResponseOverview {
	overview := &enrollment.PhaseResponseOverview{Applicable: true, Rows: make([]enrollment.PhaseResponseRow, 0, len(roster))}
	excluded := map[string]int{}
	for _, student := range roster {
		answer, responded := answers[student.ID]
		// A family that answered is always shown, whatever the rules say
		// about the child: the answer exists and the school has to see it.
		if !responded {
			if reason := phaseResponseExclusion(scope, exits, student); reason != "" {
				excluded[reason]++
				continue
			}
		}
		row := enrollment.PhaseResponseRow{
			StudentID: student.ID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, HasParentApp: apps[student.ID], Responded: responded,
		}
		if responded {
			requestID := answer.requestID
			row.RequestID, row.ChildStatus = &requestID, answer.status
			overview.Responded++
		} else if open, ok := waiting[student.ID]; ok {
			requestID := open.requestID
			row.PendingRequestID, row.ChildStatus = &requestID, open.status
		}
		overview.Rows = append(overview.Rows, row)
	}
	overview.Expected = len(overview.Rows)
	overview.Excluded = sortedPhaseResponseExclusions(excluded)
	sort.SliceStable(overview.Rows, func(i, j int) bool { return phaseResponseRowLess(overview.Rows[i], overview.Rows[j]) })
	return overview
}

// phaseResponseExclusion is why a child without an answer is left out, or
// "". A recorded care end wins over the phase's own scope.
func phaseResponseExclusion(scope phaseResponseScope, exits map[int64]bool, student enrollment.PhaseResponseStudent) string {
	if exits[student.ID] {
		return enrollment.PhaseResponseExcludedCareEnding
	}
	return scope.exclusion(student.SchoolClass)
}

// phaseResponseRowLess puts the missing answers first, then sorts by name.
func phaseResponseRowLess(a, b enrollment.PhaseResponseRow) bool {
	if a.Responded != b.Responded {
		return !a.Responded
	}
	if c := strings.Compare(strings.ToLower(a.LastName), strings.ToLower(b.LastName)); c != 0 {
		return c < 0
	}
	return strings.ToLower(a.FirstName) < strings.ToLower(b.FirstName)
}

func sortedPhaseResponseExclusions(counts map[string]int) []enrollment.PhaseResponseExclusion {
	result := make([]enrollment.PhaseResponseExclusion, 0, len(counts))
	for _, reason := range []string{
		enrollment.PhaseResponseExcludedCareEnding, enrollment.PhaseResponseExcludedGraduating, enrollment.PhaseResponseExcludedNotInScope,
	} {
		if counts[reason] > 0 {
			result = append(result, enrollment.PhaseResponseExclusion{Reason: reason, Count: counts[reason]})
		}
	}
	return result
}
