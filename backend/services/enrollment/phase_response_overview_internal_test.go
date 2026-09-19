package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// responsePhaseOwner serves one phase; every other owner call would panic on
// the nil embedded interface, which the overview must never reach.
type responsePhaseOwner struct {
	PhaseOwner
	phase *enrollmentOwner.Phase
}

func (o responsePhaseOwner) Phase(context.Context, int64) (*enrollmentOwner.Phase, error) {
	if o.phase == nil {
		return nil, sql.ErrNoRows
	}
	return o.phase, nil
}

type responseChildren struct {
	byPhase  []*enrollmentOwner.RequestChild
	byID     map[int64]*enrollmentOwner.RequestChild
	statuses []string
	idCalls  int
}

func (c *responseChildren) ChildrenByPhaseStatuses(_ context.Context, _ int64, statuses []string) ([]*enrollmentOwner.RequestChild, error) {
	c.statuses = statuses
	return c.byPhase, nil
}

func (c *responseChildren) ChildrenByID(_ context.Context, ids []int64) ([]*enrollmentOwner.RequestChild, error) {
	c.idCalls++
	result := make([]*enrollmentOwner.RequestChild, 0, len(ids))
	for _, id := range ids {
		if child, ok := c.byID[id]; ok {
			result = append(result, child)
		}
	}
	return result, nil
}

type responseRoster []PhaseResponseStudent

func (r responseRoster) ListRunningStudents(context.Context, timezone.Date) ([]PhaseResponseStudent, error) {
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

func (s responseSettings) ResolveBool(context.Context, string) (bool, error) { return false, nil }
func (s responseSettings) ResolveInt(context.Context, string) (int, error)   { return s.gradeMax, s.err }

func ptr[T any](v T) *T { return &v }

// The fixed day sits before the 2027/28 care year, so a school-year phase
// starting 2027-08-01 counts as "next year".
var responseToday = timezone.NewDate(2027, 3, 1)

func nextYearPhase() *enrollmentOwner.Phase {
	return &enrollmentOwner.Phase{
		ID: 5, Kind: enrollmentOwner.PhaseKindSchoolYear, ServiceStartDate: "2027-08-01",
		ServiceEndDate: "2028-07-31", Audience: enrollmentOwner.PhaseAudienceExistingStudents,
	}
}

func newResponseService(phase *enrollmentOwner.Phase, children *responseChildren, roster responseRoster, exits, apps responseFlags) *phaseService {
	return &phaseService{
		owner:    responsePhaseOwner{phase: phase},
		settings: responseSettings{gradeMax: 4},
		today:    func() timezone.Date { return responseToday },
		responses: &PhaseResponseSources{
			Children: children, Roster: roster, CareExits: exits, PortalAccounts: apps,
		},
	}
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
	children := &responseChildren{byPhase: []*enrollmentOwner.RequestChild{
		{ID: 10, RequestID: 100, Status: enrollmentOwner.ChildStatusSubmitted, MatchedStudentID: ptr(int64(1))},
	}}
	svc := newResponseService(nextYearPhase(), children, roster, responseFlags{4: true}, responseFlags{1: true, 2: true})

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)

	assert.True(t, overview.Applicable)
	assert.Equal(t, 4, overview.Expected)
	assert.Equal(t, 1, overview.Responded)
	assert.Equal(t, []PhaseResponseExclusion{
		{Reason: PhaseResponseExcludedCareEnding, Count: 1},
		{Reason: PhaseResponseExcludedGraduating, Count: 1},
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
	assert.Equal(t, enrollmentOwner.ChildStatusSubmitted, answered.ChildStatus)

	// withdrawn is never asked for, so a withdrawn submission cannot count.
	assert.NotContains(t, children.statuses, enrollmentOwner.ChildStatusWithdrawn)
}

func TestResponseOverview_EverySubmittedOrDecidedStateCountsAsAnswered(t *testing.T) {
	t.Parallel()

	statuses := []string{
		enrollmentOwner.ChildStatusSubmitted, enrollmentOwner.ChildStatusUnderReview,
		enrollmentOwner.ChildStatusApproved, enrollmentOwner.ChildStatusWaitlisted,
		enrollmentOwner.ChildStatusRejected,
	}
	roster := responseRoster{}
	children := &responseChildren{}
	for i, status := range statuses {
		id := int64(i + 1)
		roster = append(roster, PhaseResponseStudent{ID: id, LastName: status, SchoolClass: "1a"})
		children.byPhase = append(children.byPhase, &enrollmentOwner.RequestChild{
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

	children := &responseChildren{byPhase: []*enrollmentOwner.RequestChild{
		{ID: 10, RequestID: 100, Status: enrollmentOwner.ChildStatusAutoRenewed, MatchedStudentID: ptr(int64(1))},
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
	assert.Equal(t, enrollmentOwner.ChildStatusAutoRenewed, row.ChildStatus)
	assert.Contains(t, children.statuses, enrollmentOwner.ChildStatusAutoRenewed)
}

func TestResponseOverview_OpenRolloverRowIsNoAnswerButIsLinked(t *testing.T) {
	t.Parallel()

	phase := nextYearPhase()
	phase.Audience = enrollmentOwner.PhaseAudienceOpen
	phase.RolloverSourcePhaseID = ptr(int64(4))
	roster := responseRoster{
		{ID: 1, LastName: "Arslan", SchoolClass: "1a"},
		{ID: 2, LastName: "Becker", SchoolClass: "2a"},
	}
	children := &responseChildren{
		byPhase: []*enrollmentOwner.RequestChild{
			{ID: 20, RequestID: 200, Status: enrollmentOwner.ChildStatusPendingRenewal, RolloverSourceChildID: ptr(int64(11))},
			{ID: 21, RequestID: 201, Status: enrollmentOwner.ChildStatusSubmitted, RolloverSourceChildID: ptr(int64(12))},
		},
		byID: map[int64]*enrollmentOwner.RequestChild{
			11: {ID: 11, CreatedStudentID: ptr(int64(1))},
			12: {ID: 12, MatchedStudentID: ptr(int64(2))},
		},
	}
	svc := newResponseService(phase, children, roster, nil, nil)

	overview, err := svc.ResponseOverview(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, 2, overview.Expected)
	assert.Equal(t, 1, overview.Responded)
	assert.Equal(t, 1, children.idCalls, "rollover sources are read in one call")

	waiting := overview.Rows[0]
	assert.Equal(t, int64(1), waiting.StudentID)
	assert.False(t, waiting.Responded)
	assert.Nil(t, waiting.RequestID)
	require.NotNil(t, waiting.PendingRequestID)
	assert.Equal(t, int64(200), *waiting.PendingRequestID)
	assert.Equal(t, enrollmentOwner.ChildStatusPendingRenewal, waiting.ChildStatus)
	assert.True(t, overview.Rows[1].Responded)
}

func TestResponseOverview_ResolvesMultiGenerationRolloverSources(t *testing.T) {
	t.Parallel()

	children := &responseChildren{
		byPhase: []*enrollmentOwner.RequestChild{
			{ID: 30, RequestID: 300, Status: enrollmentOwner.ChildStatusSubmitted, RolloverSourceChildID: ptr(int64(20))},
		},
		byID: map[int64]*enrollmentOwner.RequestChild{
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
	assert.Equal(t, 2, children.idCalls, "each rollover generation is loaded in one batch")
}

func TestResponseOverview_SeveralSubmissionsCountOnceAndShowTheFirst(t *testing.T) {
	t.Parallel()

	roster := responseRoster{{ID: 1, LastName: "Arslan", SchoolClass: "1a"}}
	children := &responseChildren{byPhase: []*enrollmentOwner.RequestChild{
		{ID: 31, RequestID: 301, Status: enrollmentOwner.ChildStatusSubmitted, MatchedStudentID: ptr(int64(1))},
		{ID: 30, RequestID: 300, Status: enrollmentOwner.ChildStatusRejected, MatchedStudentID: ptr(int64(1))},
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
	children := &responseChildren{byPhase: []*enrollmentOwner.RequestChild{
		{ID: 40, RequestID: 400, Status: enrollmentOwner.ChildStatusSubmitted, MatchedStudentID: ptr(int64(1))},
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

	children := &responseChildren{byPhase: []*enrollmentOwner.RequestChild{
		{
			ID: 10, RequestID: 100, Status: enrollmentOwner.ChildStatusSubmitted,
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
	assert.Equal(t, []PhaseResponseExclusion{{Reason: PhaseResponseExcludedNotInScope, Count: 1}}, overview.Excluded)
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
	phase.Kind = enrollmentOwner.PhaseKindHoliday
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
	phase.Kind = enrollmentOwner.PhaseKindHoliday
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
	assert.Equal(t, []PhaseResponseExclusion{{Reason: PhaseResponseExcludedNotInScope, Count: 1}}, overview.Excluded)
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
	assert.Equal(t, []PhaseResponseExclusion{{Reason: PhaseResponseExcludedNotInScope, Count: 1}}, overview.Excluded)
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
		phase       *enrollmentOwner.Phase
		schoolClass string
	}{
		{
			name: "current school year",
			phase: func() *enrollmentOwner.Phase {
				phase := nextYearPhase()
				phase.Kind = enrollmentOwner.PhaseKindHoliday
				phase.EligibleSchoolClasses = []string{" 1A "}
				return phase
			}(),
			schoolClass: "1a",
		},
		{
			name: "future school year",
			phase: func() *enrollmentOwner.Phase {
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
	phase.Audience = enrollmentOwner.PhaseAudienceOpen
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
		assert.ErrorIs(t, err, ErrPhaseNotFound)
	})
	t.Run("ports not wired", func(t *testing.T) {
		t.Parallel()
		svc := &phaseService{owner: responsePhaseOwner{phase: nextYearPhase()}}
		_, err := svc.ResponseOverview(context.Background(), 5)
		assert.ErrorIs(t, err, ErrPhaseResponseOverviewUnavailable)
	})
	t.Run("grade limit cannot be resolved", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("settings down")
		svc := newResponseService(nextYearPhase(), &responseChildren{}, nil, nil, nil)
		svc.settings = responseSettings{err: boom}
		_, err := svc.ResponseOverview(context.Background(), 5)
		assert.ErrorIs(t, err, boom)
	})
}
