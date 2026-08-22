package enrollment

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// stubMatchedStudentChildRepo captures the pin writes reconcileMatchedStudent
// performs. The embedded interface panics on any other call, flagging an
// unintended dependency.
type stubMatchedStudentChildRepo struct {
	enrollmentModels.RequestChildRepository
	writes []*int64
	err    error
}

func (s *stubMatchedStudentChildRepo) UpdateMatchedStudent(_ context.Context, _ int64, studentID *int64) error {
	s.writes = append(s.writes, studentID)
	return s.err
}

// stubReEnrollAuthorizer answers the per-student re-enrollment permission probe
// for both identities (account and guardian email).
type stubReEnrollAuthorizer struct {
	granted     bool
	err         error
	gotStudents []int64
	gotEmails   []string
}

func (s *stubReEnrollAuthorizer) AccountHasStudentPermission(_ context.Context, _, studentID, _ int64, _ string) (bool, error) {
	s.gotStudents = append(s.gotStudents, studentID)
	return s.granted, s.err
}

func (s *stubReEnrollAuthorizer) GuardianEmailHasStudentPermission(_ context.Context, email string, studentID, _ int64, _ string) (bool, error) {
	s.gotStudents = append(s.gotStudents, studentID)
	s.gotEmails = append(s.gotEmails, email)
	return s.granted, s.err
}

type matchedStudentFixture struct {
	svc      *changeRequestService
	children *stubMatchedStudentChildRepo
	requests *stubMatchLockRequestRepo
	auth     *stubReEnrollAuthorizer
	req      *enrollmentModels.Request
	phase    *enrollmentModels.Phase
	row      *enrollmentModels.ChangeRequest
}

func newMatchedStudentFixture(audience string, resolved *int64, collision bool) *matchedStudentFixture {
	children := &stubMatchedStudentChildRepo{}
	requests := &stubMatchLockRequestRepo{has: collision}
	auth := &stubReEnrollAuthorizer{granted: true}
	svc := &changeRequestService{ChangeRequestServiceConfig: ChangeRequestServiceConfig{
		RequestRepo:        requests,
		RequestChildRepo:   children,
		StudentRepo:        &stubMatchStudentRepo{matchID: resolved, exists: resolved != nil},
		GuardianAuthorizer: auth,
	}}
	svc.Logger = slog.Default()

	accountID := int64(4242)
	req := &enrollmentModels.Request{GuardianAccountID: &accountID}
	req.TenantID = 77

	return &matchedStudentFixture{
		svc:      svc,
		children: children,
		requests: requests,
		auth:     auth,
		req:      req,
		phase:    &enrollmentModels.Phase{Audience: audience},
		row:      &enrollmentModels.ChangeRequest{},
	}
}

func matchedStudentChild(id int64, pin *int64) *enrollmentModels.RequestChild {
	child := &enrollmentModels.RequestChild{
		FirstName:        "Anna",
		LastName:         "Müller",
		DateOfBirth:      timezone.NewDate(2019, 4, 12),
		Status:           enrollmentModels.ChildStatusUnderReview,
		MatchedStudentID: pin,
	}
	child.ID = id
	return child
}

func matchedStudentSubmit(firstName string) SubmitChild {
	return SubmitChild{
		FirstName:   firstName,
		LastName:    "Müller",
		DateOfBirth: timezone.NewDate(2019, 4, 12),
	}
}

// An approved change request may rewrite name + birthday. The submission-time
// pin then describes a different human than the reviewed row, so it must be
// re-resolved against the new identity before the decision service renews
// anything (#1663).
func TestReconcileMatchedStudent_RepinsAfterIdentityEdit(t *testing.T) {
	t.Parallel()

	newStudent := int64(900)
	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, &newStudent, false)
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.NoError(t, err)

	require.Len(t, f.children.writes, 1, "an edited identity must rewrite the pin")
	require.NotNil(t, f.children.writes[0])
	assert.Equal(t, newStudent, *f.children.writes[0])
	assert.Equal(t, newStudent, *child.MatchedStudentID, "in-memory row must follow the write")
	assert.Equal(t, []int64{newStudent}, f.auth.gotStudents,
		"the permission probe must run against the NEW match, not the stale pin")
}

// An unchanged identity keeps its pin: dropping it would send approval down the
// fresh-create branch and duplicate the very student this audience renews.
func TestReconcileMatchedStudent_KeepsPinForUnchangedIdentity(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, &pin, false)
	child := matchedStudentChild(11, &pin)

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Anna"), map[int]string{}, true, 0)
	require.NoError(t, err)

	assert.Empty(t, f.children.writes, "an unchanged pin must not be rewritten")
	assert.Equal(t, []int64{11}, f.requests.excludedIDs,
		"the uniqueness re-check must exclude the row's own already-active pin")
}

// A pinned child whose identity changed on a phase that is no longer
// existing_students cannot be re-resolved: the stale pin must be cleared so
// approval creates a fresh record rather than renewing a stranger.
func TestReconcileMatchedStudent_ClearsStalePinWhenAudienceNoLongerMatches(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceOpen, nil, false)
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.NoError(t, err)

	require.Len(t, f.children.writes, 1)
	assert.Nil(t, f.children.writes[0], "the stale pin must be cleared")
	assert.Nil(t, child.MatchedStudentID)
}

// On an existing_students phase an edit to a child that matches no enrolled
// student is not a fresh create — it is an invalid submission for the phase.
func TestReconcileMatchedStudent_RejectsUnmatchableIdentityOnExistingStudentsPhase(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, nil, false)
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.ErrorIs(t, err, ErrChildNotEnrolled)
	assert.Empty(t, f.children.writes, "a rejected reconcile must not persist anything")
}

// The re-resolved pin is only allowed to stand when the guardian holds
// parent_portal.enrollment.submit on THAT student.
func TestReconcileMatchedStudent_RejectsUnpermittedNewMatch(t *testing.T) {
	t.Parallel()

	newStudent := int64(900)
	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, &newStudent, false)
	f.auth.granted = false
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.ErrorIs(t, err, ErrChildEnrollmentNotPermitted)
	assert.Empty(t, f.children.writes)
}

// HasActiveRequestForMatchedStudent ignores rejected rows, so another guardian
// may legitimately pin the same student while this request sits rejected.
// Reopening it to under_review puts both in competition again — the uniqueness
// guard must therefore run at reopen time, not only at insert time (#1663).
func TestReconcileMatchedStudent_RejectsReopenCollidingWithAnotherRequest(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, &pin, true)
	child := matchedStudentChild(11, &pin)
	child.Status = enrollmentModels.ChildStatusRejected
	f.row.BaseSnapshot = map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna"}}}
	f.row.ProposedSnapshot = map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna-Lena"}}}

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Anna"), map[int]string{}, true, 0)
	require.ErrorIs(t, err, ErrExistingStudentAlreadyRequested)
}

// A rejected child whose own data did NOT change stays rejected, so it never
// re-enters competition and must not be blocked by another request's pin.
func TestReconcileMatchedStudent_SkipsUniquenessForChildStayingRejected(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, &pin, true)
	child := matchedStudentChild(11, &pin)
	child.Status = enrollmentModels.ChildStatusRejected

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Anna"), map[int]string{}, true, 0)
	require.NoError(t, err)
	assert.Empty(t, f.requests.hasStudentIDs, "a child staying rejected must not be probed")
}

// Once approval materialized the student, SyncApprovedChildData owns that live
// record and the pin is history — reconciliation must not touch it.
func TestReconcileMatchedStudent_SkipsMaterializedChild(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, &pin, true)
	child := matchedStudentChild(11, &pin)
	student := int64(555)
	child.CreatedStudentID = &student

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.NoError(t, err)
	assert.Empty(t, f.children.writes)
	assert.Empty(t, f.requests.hasStudentIDs)
}

// Rollover children resolve their existing student through the rollover source
// chain, which takes precedence at approval — a pin here would be dead data.
func TestReconcileMatchedStudent_SkipsRolloverChild(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceExistingStudents, nil, false)
	child := matchedStudentChild(11, nil)
	source := int64(9)
	child.RolloverSourceChildID = &source

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.NoError(t, err)
	assert.Empty(t, f.children.writes)
}

// An unpinned child on an ordinary phase is the common case and must stay a
// pure no-op — no lookups, no probes, no writes.
func TestReconcileMatchedStudent_NoOpForUnpinnedOrdinaryChild(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentModels.PhaseAudienceOpen, nil, true)
	child := matchedStudentChild(11, nil)

	err := f.svc.reconcileMatchedStudent(context.Background(), f.req, f.phase, f.row,
		child, matchedStudentSubmit("Ben"), map[int]string{}, true, 0)
	require.NoError(t, err)
	assert.Empty(t, f.children.writes)
	assert.Empty(t, f.requests.hasStudentIDs)
}

// postApprovalChildStatus has to mirror the write loop's status decisions,
// because the uniqueness guard keys on whether the child ends up active.
func TestPostApprovalChildStatus(t *testing.T) {
	t.Parallel()

	row := &enrollmentModels.ChangeRequest{
		BaseSnapshot:     map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna"}}},
		ProposedSnapshot: map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna-Lena"}}},
	}
	child := matchedStudentChild(11, nil)

	t.Run("capacity override wins", func(t *testing.T) {
		child.Status = enrollmentModels.ChildStatusUnderReview
		assert.Equal(t, enrollmentModels.ChildStatusWaitlisted,
			postApprovalChildStatus(row, child, map[int]string{0: enrollmentModels.ChildStatusWaitlisted}, 0))
	})

	t.Run("rejected child with changed data reopens", func(t *testing.T) {
		child.Status = enrollmentModels.ChildStatusRejected
		assert.Equal(t, enrollmentModels.ChildStatusUnderReview,
			postApprovalChildStatus(row, child, map[int]string{}, 0))
	})

	t.Run("rejected child without own change stays rejected", func(t *testing.T) {
		child.Status = enrollmentModels.ChildStatusRejected
		unchanged := &enrollmentModels.ChangeRequest{
			BaseSnapshot:     row.BaseSnapshot,
			ProposedSnapshot: row.BaseSnapshot,
		}
		assert.Equal(t, enrollmentModels.ChildStatusRejected,
			postApprovalChildStatus(unchanged, child, map[int]string{}, 0))
	})

	t.Run("otherwise unchanged", func(t *testing.T) {
		child.Status = enrollmentModels.ChildStatusUnderReview
		assert.Equal(t, enrollmentModels.ChildStatusUnderReview,
			postApprovalChildStatus(row, child, map[int]string{}, 0))
	})
}

func TestPtrInt64Equal(t *testing.T) {
	t.Parallel()

	a, b := int64(5), int64(5)
	c := int64(6)
	assert.True(t, ptrInt64Equal(nil, nil))
	assert.True(t, ptrInt64Equal(&a, &b))
	assert.False(t, ptrInt64Equal(&a, &c))
	assert.False(t, ptrInt64Equal(&a, nil))
	assert.False(t, ptrInt64Equal(nil, &a))
}

// validateChildClassEligibility is the one child gate the change-request path
// reuses: eligible_school_classes may be a proper subset of
// available_school_classes, so an available-but-ineligible class must still be
// rejected there (#1663).
func TestValidateChildClassEligibility(t *testing.T) {
	t.Parallel()

	phase := &enrollmentModels.Phase{
		EligibleSchoolClasses:  []string{"2a"},
		AvailableSchoolClasses: []string{"2a", "3b"},
	}

	t.Run("eligible class passes", func(t *testing.T) {
		require.NoError(t, validateChildClassEligibility(phase,
			[]SubmitChild{{FirstName: "Kim", TargetSchoolClass: classPtr("2a")}}))
	})

	t.Run("available but ineligible class rejected", func(t *testing.T) {
		err := validateChildClassEligibility(phase,
			[]SubmitChild{{FirstName: "Kim", TargetSchoolClass: classPtr("3b")}})
		require.ErrorIs(t, err, ErrChildClassNotEligible)
	})

	t.Run("missing class rejected when the phase restricts", func(t *testing.T) {
		err := validateChildClassEligibility(phase, []SubmitChild{{FirstName: "Kim"}})
		require.ErrorIs(t, err, ErrChildClassNotEligible)
	})

	t.Run("unrestricted phase accepts anything", func(t *testing.T) {
		require.NoError(t, validateChildClassEligibility(&enrollmentModels.Phase{},
			[]SubmitChild{{FirstName: "Kim", TargetSchoolClass: classPtr("9z")}}))
	})
}
