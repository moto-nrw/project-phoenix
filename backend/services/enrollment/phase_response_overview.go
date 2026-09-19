package enrollment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Reasons a child of the school is left out of a phase's response overview
// (#3379). The overview names them with a count so the denominator stays
// explainable: a school that sees "80 von 96" must be able to tell why it is
// not 100.
const (
	PhaseResponseExcludedCareEnding = "care_ending"
	PhaseResponseExcludedGraduating = "graduating"
	PhaseResponseExcludedNotInScope = "not_in_scope"
)

// ErrPhaseResponseOverviewUnavailable reports a service built without the
// read ports of the overview. The factory always wires them.
var ErrPhaseResponseOverviewUnavailable = errors.New("phase response overview is not configured")

// PhaseResponseStudent is one child of the school's current roster as the
// overview needs it: identity, class and nothing else.
type PhaseResponseStudent struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// PhaseResponseRoster lists the children whose care is running on the given
// calendar day. Graduates are never part of it.
type PhaseResponseRoster interface {
	ListRunningStudents(ctx context.Context, today timezone.Date) ([]PhaseResponseStudent, error)
}

// PhaseResponseCareExits reports which of the given children have a recorded
// care end ("Betreuung beenden"). Such a child is leaving and is not asked
// to re-enroll.
type PhaseResponseCareExits interface {
	StudentsWithCareExit(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
}

// PhaseResponsePortalAccounts reports which of the given children have at
// least one guardian who uses the parents app. It is a contact hint only and
// never enters the response figure.
type PhaseResponsePortalAccounts interface {
	StudentsWithPortalGuardian(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
}

// PhaseResponseChildren reads the submitted children of a phase. The
// Enrollment owner implements it.
type PhaseResponseChildren interface {
	ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*enrollmentOwner.RequestChild, error)
	ChildrenByID(ctx context.Context, ids []int64) ([]*enrollmentOwner.RequestChild, error)
}

// PhaseResponseSources bundles the read ports of the overview.
type PhaseResponseSources struct {
	Children       PhaseResponseChildren
	Roster         PhaseResponseRoster
	CareExits      PhaseResponseCareExits
	PortalAccounts PhaseResponsePortalAccounts
}

func (s *PhaseResponseSources) complete() bool {
	return s != nil && s.Children != nil && s.Roster != nil && s.CareExits != nil && s.PortalAccounts != nil
}

// PhaseResponseRow is one expected child and, when the family answered, the
// submission that counts as the answer.
type PhaseResponseRow struct {
	StudentID        int64
	FirstName        string
	LastName         string
	SchoolClass      string
	HasParentApp     bool
	Responded        bool
	RequestID        *int64
	ChildStatus      string
	PendingRequestID *int64
}

// PhaseResponseExclusion counts the children left out for one reason.
type PhaseResponseExclusion struct {
	Reason string
	Count  int
}

// PhaseResponseOverview is the response of the existing children to one
// phase. Applicable is false for a phase that does not pin submissions to
// existing children; the figures are then meaningless and stay empty.
type PhaseResponseOverview struct {
	Applicable bool
	Expected   int
	Responded  int
	Rows       []PhaseResponseRow
	Excluded   []PhaseResponseExclusion
}

// phaseResponseCountedStatuses are the child statuses that count as "the
// family answered". Sven's question in #3379 is about handing the form in,
// not about the school's decision, so every submitted state counts whatever
// the school did with it afterwards.
//
// Deliberately absent: withdrawn (the family took the answer back) and all
// rollover-intermediate statuses. In particular, auto_renewed stays open
// until the deadline worker turns it into submitted or approved: the family
// can still decline an opt-out rollover.
var phaseResponseCountedStatuses = []string{
	enrollmentOwner.ChildStatusSubmitted,
	enrollmentOwner.ChildStatusUnderReview,
	enrollmentOwner.ChildStatusApproved,
	enrollmentOwner.ChildStatusWaitlisted,
	enrollmentOwner.ChildStatusRejected,
}

// phaseResponseWaitingStatuses are rollover rows that exist for a child but
// are no answer. The overview links them so the school finds the open row.
var phaseResponseWaitingStatuses = []string{
	enrollmentOwner.ChildStatusPendingRenewal,
	enrollmentOwner.ChildStatusAutoRenewed,
	enrollmentOwner.ChildStatusPendingAdminReview,
}

// phaseResponseApplicable says whether submissions of this phase carry a
// reference to the existing child. Only the existing_students audience pins
// matched_student_id at submit, and only a rollover copies a child forward.
// Every other phase stores name and birthday alone, and guessing the child
// from a name would put wrong families on a call list.
func phaseResponseApplicable(phase *enrollmentOwner.Phase) bool {
	return phase.Audience == enrollmentOwner.PhaseAudienceExistingStudents ||
		phase.RolloverSourcePhaseID != nil
}

func (s *phaseService) ResponseOverview(ctx context.Context, phaseID int64) (*PhaseResponseOverview, error) {
	if !s.responses.complete() {
		return nil, ErrPhaseResponseOverviewUnavailable
	}
	phase, err := s.GetByID(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	if !phaseResponseApplicable(phase) {
		return &PhaseResponseOverview{Rows: []PhaseResponseRow{}, Excluded: []PhaseResponseExclusion{}}, nil
	}
	answers, waiting, err := s.phaseResponseLinks(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	today := s.todayDate()
	roster, err := s.responses.Roster.ListRunningStudents(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("phase %d response overview: list roster: %w", phaseID, err)
	}
	roster = phaseResponseRosterWithAnswers(roster, answers)
	ids := make([]int64, 0, len(roster))
	for _, student := range roster {
		ids = append(ids, student.ID)
	}
	exits, err := s.responses.CareExits.StudentsWithCareExit(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("phase %d response overview: list care exits: %w", phaseID, err)
	}
	apps, err := s.responses.PortalAccounts.StudentsWithPortalGuardian(ctx, ids)
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

func (s *phaseService) phaseResponseGradeMax(ctx context.Context) (int, error) {
	if s.settings == nil {
		return schoolclass.DefaultGradeLevelMax, nil
	}
	value, err := s.settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
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
func (s *phaseService) phaseResponseLinks(ctx context.Context, phaseID int64) (answers, waiting map[int64]phaseResponseLink, err error) {
	statuses := append(append([]string{}, phaseResponseCountedStatuses...), phaseResponseWaitingStatuses...)
	children, err := s.responses.Children.ChildrenByPhaseStatuses(ctx, phaseID, statuses)
	if err != nil {
		return nil, nil, fmt.Errorf("phase %d response overview: list children: %w", phaseID, err)
	}
	sources, err := s.phaseResponseRolloverSources(ctx, children)
	if err != nil {
		return nil, nil, fmt.Errorf("phase %d response overview: list rollover sources: %w", phaseID, err)
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

func phaseResponseChildSchoolClass(child *enrollmentOwner.RequestChild) string {
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

func (s *phaseService) phaseResponseRolloverSources(ctx context.Context, children []*enrollmentOwner.RequestChild) (map[int64]*enrollmentOwner.RequestChild, error) {
	ids := make([]int64, 0)
	queued := make(map[int64]struct{})
	for _, child := range children {
		if child.RolloverSourceChildID != nil && phaseResponsePinnedStudent(child) == 0 {
			id := *child.RolloverSourceChildID
			if id > 0 {
				if _, seen := queued[id]; !seen {
					queued[id] = struct{}{}
					ids = append(ids, id)
				}
			}
		}
	}
	result := make(map[int64]*enrollmentOwner.RequestChild, len(ids))
	for len(ids) > 0 {
		sources, err := s.responses.Children.ChildrenByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		ids = nil
		for _, source := range sources {
			result[source.ID] = source
			if source.RolloverSourceChildID == nil || phaseResponsePinnedStudent(source) != 0 {
				continue
			}
			id := *source.RolloverSourceChildID
			if id > 0 {
				if _, seen := queued[id]; !seen {
					queued[id] = struct{}{}
					ids = append(ids, id)
				}
			}
		}
	}
	return result, nil
}

func phaseResponsePinnedStudent(child *enrollmentOwner.RequestChild) int64 {
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
func phaseResponseStudentID(child *enrollmentOwner.RequestChild, sources map[int64]*enrollmentOwner.RequestChild) int64 {
	seen := map[int64]struct{}{child.ID: struct{}{}}
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

func newPhaseResponseScope(phase *enrollmentOwner.Phase, today timezone.Date, gradeMax int) phaseResponseScope {
	scope := phaseResponseScope{
		advanceGrade: phase.Kind == enrollmentOwner.PhaseKindSchoolYear &&
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
		if class = schoolclass.Normalize(class); class != "" {
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
	grade, err := strconv.Atoi(schoolclass.GradePrefix(class))
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
	class = schoolclass.Normalize(class)
	grade, ok := phaseResponseGrade(class)
	if s.advanceGrade && ok {
		if grade >= s.gradeMax {
			return PhaseResponseExcludedGraduating
		}
		grade++
		class = phaseResponseClassWithGrade(class, grade)
	}
	if len(s.classes) > 0 {
		if _, eligible := s.classes[class]; !eligible {
			return PhaseResponseExcludedNotInScope
		}
	}
	if !ok {
		return ""
	}
	if len(s.grades) == 0 {
		return ""
	}
	if _, eligible := s.grades[grade]; !eligible {
		return PhaseResponseExcludedNotInScope
	}
	return ""
}

func phaseResponseClassWithGrade(class string, grade int) string {
	class = schoolclass.Normalize(class)
	prefix := schoolclass.GradePrefix(class)
	index := strings.Index(class, prefix)
	if index < 0 {
		return class
	}
	return class[:index] + strconv.Itoa(grade) + class[index+len(prefix):]
}

func phaseResponseRosterWithAnswers(roster []PhaseResponseStudent, answers map[int64]phaseResponseLink) []PhaseResponseStudent {
	present := make(map[int64]struct{}, len(roster))
	for _, student := range roster {
		present[student.ID] = struct{}{}
	}
	for studentID, answer := range answers {
		if _, exists := present[studentID]; exists {
			continue
		}
		roster = append(roster, PhaseResponseStudent{
			ID: studentID, FirstName: answer.firstName, LastName: answer.lastName, SchoolClass: answer.schoolClass,
		})
	}
	return roster
}

func buildPhaseResponseOverview(
	roster []PhaseResponseStudent,
	scope phaseResponseScope,
	exits, apps map[int64]bool,
	answers, waiting map[int64]phaseResponseLink,
) *PhaseResponseOverview {
	overview := &PhaseResponseOverview{Applicable: true, Rows: make([]PhaseResponseRow, 0, len(roster))}
	excluded := map[string]int{}
	for _, student := range roster {
		answer, responded := answers[student.ID]
		// A family that answered is always shown, whatever the rules say
		// about the child: the answer exists and the school has to see it.
		if !responded {
			reason := scope.exclusion(student.SchoolClass)
			if exits[student.ID] {
				reason = PhaseResponseExcludedCareEnding
			}
			if reason != "" {
				excluded[reason]++
				continue
			}
		}
		row := PhaseResponseRow{
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
	sort.SliceStable(overview.Rows, func(i, j int) bool {
		a, b := overview.Rows[i], overview.Rows[j]
		if a.Responded != b.Responded {
			return !a.Responded
		}
		if c := strings.Compare(strings.ToLower(a.LastName), strings.ToLower(b.LastName)); c != 0 {
			return c < 0
		}
		return strings.ToLower(a.FirstName) < strings.ToLower(b.FirstName)
	})
	return overview
}

func sortedPhaseResponseExclusions(counts map[string]int) []PhaseResponseExclusion {
	result := make([]PhaseResponseExclusion, 0, len(counts))
	for _, reason := range []string{
		PhaseResponseExcludedCareEnding, PhaseResponseExcludedGraduating, PhaseResponseExcludedNotInScope,
	} {
		if counts[reason] > 0 {
			result = append(result, PhaseResponseExclusion{Reason: reason, Count: counts[reason]})
		}
	}
	return result
}
