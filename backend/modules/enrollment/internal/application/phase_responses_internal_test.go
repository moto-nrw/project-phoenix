package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var errNoPhase = errors.New("phase not stored")

// responsePhaseOwner serves one phase and its submitted children; every other
// owner call would panic on the nil embedded interface, which the overview
// must never reach.
type responsePhaseOwner struct {
	PhaseRecords
	phase    *enrollment.Phase
	children *responseChildren
}

func (o responsePhaseOwner) Phase(context.Context, int64) (*enrollment.Phase, error) {
	if o.phase == nil {
		return nil, errNoPhase
	}
	return o.phase, nil
}

func (o responsePhaseOwner) PhaseResponseChildren(ctx context.Context, phaseID int64, statuses []string) ([]enrollment.PhaseResponseChild, error) {
	return o.children.PhaseResponseChildren(ctx, phaseID, statuses)
}

type responseChildren struct {
	byPhase    []*enrollment.RequestChild
	byID       map[int64]*enrollment.RequestChild
	statuses   []string
	phaseCalls int
}

func (c *responseChildren) PhaseResponseChildren(_ context.Context, _ int64, statuses []string) ([]enrollment.PhaseResponseChild, error) {
	c.statuses = statuses
	c.phaseCalls++
	result := make([]enrollment.PhaseResponseChild, 0, len(c.byPhase)+len(c.byID))
	for _, child := range c.byPhase {
		result = append(result, enrollment.PhaseResponseChild{Child: child, IsPhaseChild: true})
	}
	seen := make(map[int64]struct{})
	for _, phaseChild := range c.byPhase {
		id := int64(0)
		if phaseChild.RolloverSourceChildID != nil {
			id = *phaseChild.RolloverSourceChildID
		}
		for child, ok := c.byID[id]; ok && child != nil; {
			if _, repeated := seen[child.ID]; repeated {
				break
			}
			seen[child.ID] = struct{}{}
			result = append(result, enrollment.PhaseResponseChild{Child: child})
			if child.RolloverSourceChildID == nil {
				break
			}
			child, ok = c.byID[*child.RolloverSourceChildID]
		}
	}
	return result, nil
}

type responseRoster []enrollment.PhaseResponseStudent

func (r responseRoster) ListRunningStudents(context.Context, calendar.Date) ([]enrollment.PhaseResponseStudent, error) {
	return r, nil
}

type responseFlags map[int64]bool

func (f responseFlags) StudentsWithCareExit(context.Context, []int64) (map[int64]bool, error) {
	return f, nil
}

func (f responseFlags) StudentsWithPortalGuardian(context.Context, []int64) (map[int64]bool, error) {
	return f, nil
}

type responseSettings struct {
	gradeMax int
	err      error
}

func (s responseSettings) CollectGradeLevel(context.Context) (bool, error)  { return false, nil }
func (s responseSettings) CollectSchoolClass(context.Context) (bool, error) { return false, nil }
func (s responseSettings) GradeLevelMax(context.Context) (int, error)       { return s.gradeMax, s.err }
func (s responseSettings) LockClassCollectionPair(context.Context) error    { return nil }

func ptr[T any](v T) *T { return &v }

// The fixed day sits before the 2027/28 care year, so a school-year phase
// starting 2027-08-01 counts as "next year".
var responseToday = calendar.NewDate(2027, 3, 1)

func nextYearPhase() *enrollment.Phase {
	return &enrollment.Phase{
		ID: 5, Kind: enrollment.PhaseKindSchoolYear, ServiceStartDate: "2027-08-01",
		ServiceEndDate: "2028-07-31", Audience: enrollment.PhaseAudienceExistingStudents,
	}
}

func responseDependencies(phase *enrollment.Phase, children *responseChildren) PhaseDependencies {
	return PhaseDependencies{
		Records:  responsePhaseOwner{phase: phase, children: children},
		Settings: responseSettings{gradeMax: 4},
		Today:    func() calendar.Date { return responseToday },
		Runtime:  Runtime{NotFound: func(err error) bool { return errors.Is(err, errNoPhase) }},
	}
}

func newResponseService(phase *enrollment.Phase, children *responseChildren, roster responseRoster, exits, apps responseFlags) *Phases {
	deps := responseDependencies(phase, children)
	deps.Responses = &PhaseResponseSources{Roster: roster, CareExits: exits, PortalAccounts: apps}
	return NewPhases(deps)
}

func TestResponseOverview_CountsAnswersAgainstTheExpectedChildren(t *testing.T) {
	t.Parallel()

	roster := responseRoster{
		{ID: 1, FirstName: "Ben", LastName: "Yilmaz", SchoolClass: "1a"},
		{ID: 2, FirstName: "Mia", LastName: "Arslan", SchoolClass: "2b"},
		{ID: 3, FirstName: "Tom", LastName: "Becker", SchoolClass: "4a"},    // leaves the school
		{ID: 4, FirstName: "Lea", LastName: "Demir", SchoolClass: "3a"},     // care end recorded
		{ID: 5, FirstName: "Ole", LastName: "Ernst", SchoolClass: "Bienen"}, // no grade: stays expected
		{ID: 6, FirstName: "Ida", LastName: "Fuchs", SchoolClass: "1a"},     // withdrew
	}
	children := &responseChildren{byPhase: []*enrollment.RequestChild{
		{ID: 10, RequestID: 100, Status: enrollment.ChildStatusSubmitted, MatchedStudentID: ptr(int64(1))},
	}}
	svc := newResponseService(nextYearPhase(), children, roster, responseFlags{4: true}, responseFlags{1: true, 2: true})

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)

	assert.True(t, overview.Applicable)
	assert.Equal(t, 4, overview.Expected)
	assert.Equal(t, 1, overview.Responded)
	assert.Equal(t, []enrollment.PhaseResponseExclusion{
		{Reason: enrollment.PhaseResponseExcludedCareEnding, Count: 1},
		{Reason: enrollment.PhaseResponseExcludedGraduating, Count: 1},
	}, overview.Excluded)

	// Missing answers first, each group by last name.
	require.Len(t, overview.Rows, 4)
	assert.Equal(t, []int64{2, 5, 6, 1}, []int64{
		overview.Rows[0].StudentID, overview.Rows[1].StudentID, overview.Rows[2].StudentID, overview.Rows[3].StudentID,
	})
	assert.True(t, overview.Rows[0].HasParentApp)
	assert.False(t, overview.Rows[1].HasParentApp)
	answered := overview.Rows[3]
	assert.True(t, answered.Responded)
	require.NotNil(t, answered.RequestID)
	assert.Equal(t, int64(100), *answered.RequestID)
	assert.Equal(t, enrollment.ChildStatusSubmitted, answered.ChildStatus)

	// withdrawn is never asked for, so a withdrawn submission cannot count.
	assert.NotContains(t, children.statuses, enrollment.ChildStatusWithdrawn)
}

func TestResponseOverview_EverySubmittedOrDecidedStateCountsAsAnswered(t *testing.T) {
	t.Parallel()

	statuses := []string{
		enrollment.ChildStatusSubmitted, enrollment.ChildStatusUnderReview,
		enrollment.ChildStatusApproved, enrollment.ChildStatusWaitlisted,
		enrollment.ChildStatusRejected,
	}
	roster := responseRoster{}
	children := &responseChildren{}
	for i, status := range statuses {
		id := int64(i + 1)
		roster = append(roster, enrollment.PhaseResponseStudent{ID: id, LastName: status, SchoolClass: "1a"})
		children.byPhase = append(children.byPhase, &enrollment.RequestChild{
			ID: id, RequestID: id, Status: status, MatchedStudentID: ptr(id),
		})
	}
	svc := newResponseService(nextYearPhase(), children, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, len(statuses), overview.Expected)
	assert.Equal(t, len(statuses), overview.Responded)
}

func TestResponseOverview_AutoRenewedIsOpenButLinked(t *testing.T) {
	t.Parallel()

	children := &responseChildren{byPhase: []*enrollment.RequestChild{
		{ID: 10, RequestID: 100, Status: enrollment.ChildStatusAutoRenewed, MatchedStudentID: ptr(int64(1))},
	}}
	svc := newResponseService(nextYearPhase(), children, responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: "1a"}}, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 1, overview.Expected)
	assert.Zero(t, overview.Responded)
	require.Len(t, overview.Rows, 1)
	row := overview.Rows[0]
	assert.False(t, row.Responded)
	assert.Nil(t, row.RequestID)
	require.NotNil(t, row.PendingRequestID)
	assert.Equal(t, int64(100), *row.PendingRequestID)
	assert.Equal(t, enrollment.ChildStatusAutoRenewed, row.ChildStatus)
	assert.Contains(t, children.statuses, enrollment.ChildStatusAutoRenewed)
}

func TestResponseOverview_OpenRolloverRowIsNoAnswerButIsLinked(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.Audience = enrollment.PhaseAudienceOpen
	phase.RolloverSourcePhaseID = ptr(int64(4))
	roster := responseRoster{
		{ID: 1, LastName: "Arslan", SchoolClass: "1a"},
		{ID: 2, LastName: "Becker", SchoolClass: "2a"},
	}
	children := &responseChildren{
		byPhase: []*enrollment.RequestChild{
			{ID: 20, RequestID: 200, Status: enrollment.ChildStatusPendingRenewal, RolloverSourceChildID: ptr(int64(11))},
			{ID: 21, RequestID: 201, Status: enrollment.ChildStatusSubmitted, RolloverSourceChildID: ptr(int64(12))},
		},
		byID: map[int64]*enrollment.RequestChild{
			11: {ID: 11, CreatedStudentID: ptr(int64(1))},
			12: {ID: 12, MatchedStudentID: ptr(int64(2))},
		},
	}
	svc := newResponseService(phase, children, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 2, overview.Expected)
	assert.Equal(t, 1, overview.Responded)
	assert.Equal(t, 1, children.phaseCalls, "phase children and rollover sources are read together")

	waiting := overview.Rows[0]
	assert.Equal(t, int64(1), waiting.StudentID)
	assert.False(t, waiting.Responded)
	assert.Nil(t, waiting.RequestID)
	require.NotNil(t, waiting.PendingRequestID)
	assert.Equal(t, int64(200), *waiting.PendingRequestID)
	assert.Equal(t, enrollment.ChildStatusPendingRenewal, waiting.ChildStatus)
	assert.True(t, overview.Rows[1].Responded)
}

func TestResponseOverview_ResolvesMultiGenerationRolloverSources(t *testing.T) {
	t.Parallel()

	children := &responseChildren{
		byPhase: []*enrollment.RequestChild{
			{ID: 30, RequestID: 300, Status: enrollment.ChildStatusSubmitted, RolloverSourceChildID: ptr(int64(20))},
		},
		byID: map[int64]*enrollment.RequestChild{
			20: {ID: 20, RolloverSourceChildID: ptr(int64(10))},
			10: {ID: 10, CreatedStudentID: ptr(int64(1))},
		},
	}
	svc := newResponseService(nextYearPhase(), children, responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: "1a"}}, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 1, overview.Responded)
	require.Len(t, overview.Rows, 1)
	assert.True(t, overview.Rows[0].Responded)
	assert.Equal(t, 1, children.phaseCalls, "the complete rollover chain is loaded in one call")
}

func TestResponseOverview_SeveralSubmissionsCountOnceAndShowTheFirst(t *testing.T) {
	t.Parallel()

	roster := responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: "1a"}}
	children := &responseChildren{byPhase: []*enrollment.RequestChild{
		{ID: 31, RequestID: 301, Status: enrollment.ChildStatusSubmitted, MatchedStudentID: ptr(int64(1))},
		{ID: 30, RequestID: 300, Status: enrollment.ChildStatusRejected, MatchedStudentID: ptr(int64(1))},
	}}
	svc := newResponseService(nextYearPhase(), children, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 1, overview.Responded)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(300), *overview.Rows[0].RequestID)
}

func TestResponseOverview_AnsweredChildIsShownEvenWhenTheRulesWouldDropIt(t *testing.T) {
	t.Parallel()

	roster := responseRoster{{ID: 1, LastName: "Becker", SchoolClass: "4a"}}
	children := &responseChildren{byPhase: []*enrollment.RequestChild{
		{ID: 40, RequestID: 400, Status: enrollment.ChildStatusSubmitted, MatchedStudentID: ptr(int64(1))},
	}}
	svc := newResponseService(nextYearPhase(), children, roster, responseFlags{1: true}, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 1, overview.Expected)
	assert.Equal(t, 1, overview.Responded)
	assert.Empty(t, overview.Excluded)
}

func TestResponseOverview_AnsweredChildStaysVisibleOutsideTheRunningRoster(t *testing.T) {
	t.Parallel()

	children := &responseChildren{byPhase: []*enrollment.RequestChild{
		{
			ID: 10, RequestID: 100, Status: enrollment.ChildStatusSubmitted,
			MatchedStudentID: ptr(int64(1)), FirstName: "Mia", LastName: "Arslan",
			TargetSchoolClass: ptr("2a"),
		},
	}}
	svc := newResponseService(nextYearPhase(), children, nil, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 1, overview.Expected)
	assert.Equal(t, 1, overview.Responded)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
	assert.Equal(t, "Mia", overview.Rows[0].FirstName)
	assert.Equal(t, "2a", overview.Rows[0].SchoolClass)
}

func TestResponseOverview_GradeRestrictionComparesNextYearsGrade(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.EligibleGradeLevels = []int{3}
	roster := responseRoster{
		{ID: 1, LastName: "Arslan", SchoolClass: "2a"}, // grade 3 next year
		{ID: 2, LastName: "Becker", SchoolClass: "3a"}, // grade 4 next year
	}
	svc := newResponseService(phase, &responseChildren{}, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
	assert.Equal(t, []enrollment.PhaseResponseExclusion{{Reason: enrollment.PhaseResponseExcludedNotInScope, Count: 1}}, overview.Excluded)
}

func TestResponseOverview_RolloverWithoutGradeBumpUsesCurrentGrade(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.RolloverSourcePhaseID = ptr(int64(4))
	phase.RolloverBumpsGrade = false
	phase.EligibleGradeLevels = []int{4}
	svc := newResponseService(phase, &responseChildren{}, responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: "4a"}}, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
	assert.Empty(t, overview.Excluded)
}

func TestResponseOverview_HolidayPhaseKeepsTheTopGradeAndTodaysGrade(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.Kind = enrollment.PhaseKindHoliday
	phase.EligibleSchoolClasses = []string{"4a", "4b"}
	roster := responseRoster{
		{ID: 1, LastName: "Arslan", SchoolClass: "4a"},
		{ID: 2, LastName: "Becker", SchoolClass: "3a"},
	}
	svc := newResponseService(phase, &responseChildren{}, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
}

func TestResponseOverview_ConcreteClassRestrictionExcludesOtherClassesOfTheSameGrade(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.Kind = enrollment.PhaseKindHoliday
	phase.EligibleSchoolClasses = []string{"2a"}
	roster := responseRoster{
		{ID: 1, LastName: "Arslan", SchoolClass: "2a"},
		{ID: 2, LastName: "Becker", SchoolClass: "2b"},
	}
	svc := newResponseService(phase, &responseChildren{}, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
	assert.Equal(t, []enrollment.PhaseResponseExclusion{{Reason: enrollment.PhaseResponseExcludedNotInScope, Count: 1}}, overview.Excluded)
}

func TestResponseOverview_FutureSchoolYearComparesConcreteClassesInTheTargetYear(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.EligibleSchoolClasses = []string{"2a"}
	roster := responseRoster{
		{ID: 1, LastName: "Arslan", SchoolClass: "1a"},
		{ID: 2, LastName: "Becker", SchoolClass: "1b"},
	}
	svc := newResponseService(phase, &responseChildren{}, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
	assert.Equal(t, []enrollment.PhaseResponseExclusion{{Reason: enrollment.PhaseResponseExcludedNotInScope, Count: 1}}, overview.Excluded)
}

func TestResponseOverview_FutureSchoolYearAdvancesAClassWithTextBeforeItsGrade(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.EligibleSchoolClasses = []string{"Klasse 2a"}
	svc := newResponseService(phase, &responseChildren{}, responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: "Klasse 1a"}}, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, int64(1), overview.Rows[0].StudentID)
}

func TestResponseOverview_NormalizesConcreteClassRestrictions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		phase       *enrollment.Phase
		schoolClass string
	}{
		{
			name: "current school year",
			phase: func() *enrollment.Phase {
				phase := nextYearPhase()
				phase.Kind = enrollment.PhaseKindHoliday
				phase.EligibleSchoolClasses = []string{" 1A "}
				return phase
			}(),
			schoolClass: "1a",
		},
		{
			name: "future school year",
			phase: func() *enrollment.Phase {
				phase := nextYearPhase()
				phase.EligibleSchoolClasses = []string{" Klasse 2A "}
				return phase
			}(),
			schoolClass: " Klasse 1a ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newResponseService(tt.phase, &responseChildren{}, responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: tt.schoolClass}}, nil, nil)

			overview, err := svc.ResponseOverview(context.Background(), 5)
			require.NoError(t, err)
			require.Len(t, overview.Rows, 1)
			assert.Equal(t, int64(1), overview.Rows[0].StudentID)
		})
	}
}

func TestResponseOverview_PhaseWithoutChildReferenceIsNotApplicable(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.Audience = enrollment.PhaseAudienceOpen
	svc := newResponseService(phase, &responseChildren{}, responseRoster{{ID: 1, SchoolClass: "1a"}}, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.False(t, overview.Applicable)
	assert.Zero(t, overview.Expected)
	assert.Empty(t, overview.Rows)
}

func TestResponseOverview_Errors(t *testing.T) {
	t.Parallel()

	t.Run("unknown phase", func(t *testing.T) {
		t.Parallel()
		svc := newResponseService(nil, &responseChildren{}, nil, nil, nil)
		_, err := svc.ResponseOverview(context.Background(), 5)
		assert.ErrorIs(t, err, enrollment.ErrPhaseNotFound)
	})
	t.Run("ports not wired", func(t *testing.T) {
		t.Parallel()
		svc := NewPhases(responseDependencies(nextYearPhase(), &responseChildren{}))
		_, err := svc.ResponseOverview(context.Background(), 5)
		assert.ErrorIs(t, err, enrollment.ErrPhaseResponseOverviewUnavailable)
	})
	t.Run("grade limit cannot be resolved", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("settings down")
		deps := responseDependencies(nextYearPhase(), &responseChildren{})
		deps.Settings = responseSettings{err: boom}
		deps.Responses = &PhaseResponseSources{Roster: responseRoster(nil), CareExits: responseFlags(nil), PortalAccounts: responseFlags(nil)}
		_, err := NewPhases(deps).ResponseOverview(context.Background(), 5)
		assert.ErrorIs(t, err, boom)
	})
}
