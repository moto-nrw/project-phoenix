package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type approvalReadStub struct {
	IntakeChildren
	ChangeRequestRecords
	ApprovalLateInvites
	DecisionLateInvites
	children                      map[int64]*enrollmentNative.RequestChild
	requests                      []*enrollmentNative.Request
	invites                       []*enrollmentNative.LateInvite
	batches                       [][]int64
	requestBatches, inviteBatches int
	err                           error
}

func (s *approvalReadStub) ChildrenByID(_ context.Context, ids []int64) ([]*enrollmentNative.RequestChild, error) {
	s.batches = append(s.batches, ids)
	var rows []*enrollmentNative.RequestChild
	for _, id := range ids {
		if row := s.children[id]; row != nil {
			rows = append(rows, row)
		}
	}
	return rows, s.err
}

func (s *approvalReadStub) RequestsByID(_ context.Context, _ []int64) ([]*enrollmentNative.Request, error) {
	s.requestBatches++
	return s.requests, s.err
}

func (s *approvalReadStub) LateInvitesByUsedRequestIDs(_ context.Context, _ []int64) ([]*enrollmentNative.LateInvite, error) {
	s.inviteBatches++
	return s.invites, s.err
}

func TestApprovalAuthorityBatchesRolloverOriginsAndReusesIdentities(t *testing.T) {
	t.Parallel()
	for _, count := range []int{1, 8} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			t.Parallel()
			store := &approvalReadStub{children: map[int64]*enrollmentNative.RequestChild{}}
			request := &enrollmentNative.Request{ID: 90, GuardianEmail: "edited@example.test"}
			run := &approval{req: request, prepared: &proposal{phase: &enrollmentNative.Phase{}}}
			for i := range count {
				originID, middleID, childID, studentID := int64(100+i), int64(200+i), int64(300+i), int64(400+i)
				requestID := int64(500 + i%2)
				store.children[originID] = &enrollmentNative.RequestChild{ID: originID, RequestID: requestID}
				store.children[middleID] = &enrollmentNative.RequestChild{ID: middleID, RolloverSourceChildID: &originID}
				run.children = append(run.children, &enrollmentNative.RequestChild{ID: childID, Status: enrollmentNative.ChildStatusApproved, CreatedStudentID: &studentID, RolloverSourceChildID: &middleID})
				if i < 2 {
					store.requests = append(store.requests, &enrollmentNative.Request{ID: requestID, SubmissionSource: enrollmentNative.RequestSourceLateInvite, GuardianEmail: "changed-origin@example.test"})
					store.invites = append(store.invites, &enrollmentNative.LateInvite{UsedRequestID: &requestID, GuardianEmail: "invited@example.test"})
				}
			}
			hold := &stubReEnrollAuthorizer{granted: true}
			svc := NewChangeRequests(ChangeRequestDependencies{Settings: intakeSettingsFake{}, Requests: store, Children: store, LateInvites: store, LateInviteIdentities: store, GuardianHold: hold, Decisions: &Decisions{}, ParentsURL: "http://localhost"})
			require.NoError(t, svc.holdApprovalAuthority(context.Background(), run))
			require.Len(t, store.batches, 2)
			require.Len(t, store.batches[0], count)
			require.Len(t, store.batches[1], count)
			require.Equal(t, 1, store.requestBatches)
			require.Equal(t, 1, store.inviteBatches)
			heldStudents := len(hold.gotStudents)
			ctx := context.WithValue(context.Background(), approvalAuthorityReadsKey{}, run.reads)
			decisions := &Decisions{deps: DecisionDependencies{LateInvites: store, GuardianHold: hold}}
			for _, child := range run.children {
				sync := &approvedChildSync{request: request, child: child}
				identity, err := decisions.syncAuthorityRequest(ctx, sync)
				require.NoError(t, err)
				require.Equal(t, enrollmentNative.RequestSourceLateInvite, identity.SubmissionSource)
				lapsed, err := decisions.childEnrollmentOnly(ctx, sync)
				require.NoError(t, err)
				require.False(t, lapsed)
				guardian, err := decisions.guardianIdentityRequest(ctx, identity)
				require.NoError(t, err)
				require.Equal(t, "invited@example.test", guardian.GuardianEmail)
			}
			require.Len(t, store.batches, 2)
			require.Equal(t, 1, store.requestBatches)
			require.Equal(t, 1, store.inviteBatches)
			require.Len(t, hold.gotStudents, heldStudents, "child synchronization must reuse the transaction-held grants")
			for _, email := range hold.gotEmails {
				require.Equal(t, "invited@example.test", email)
			}
		})
	}
}

func TestApprovalRolloverOriginsPreserveMissingAndInvalidChainHandling(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing child", "missing request", "cycle", "read error"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			sourceID := int64(10)
			start := &enrollmentNative.RequestChild{ID: 20, RolloverSourceChildID: &sourceID}
			store := &approvalReadStub{children: map[int64]*enrollmentNative.RequestChild{}}
			switch scenario {
			case "missing request":
				store.children[sourceID] = &enrollmentNative.RequestChild{ID: sourceID, RequestID: 30}
			case "cycle":
				store.children[sourceID] = &enrollmentNative.RequestChild{ID: sourceID, RolloverSourceChildID: &sourceID}
			case "read error":
				store.err = errors.New("storage unavailable")
			}
			origins, err := loadRolloverOrigins(context.Background(), store, store, []*enrollmentNative.RequestChild{start})
			switch scenario {
			case "cycle":
				require.ErrorContains(t, err, "exceeds 50 phases")
			case "read error":
				require.ErrorIs(t, err, store.err)
			default:
				require.NoError(t, err)
				require.Contains(t, origins, start.ID)
				require.Nil(t, origins[start.ID])
			}
			require.Len(t, store.batches, 1)
		})
	}
}

// stubMatchedStudentChildRepo captures the pin writes reconcileMatchedStudent
// performs. The embedded interface panics on any other call, flagging an
// unintended dependency.
type stubMatchedStudentChildRepo struct {
	IntakeChildren
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

// The approval holds the identity's authority behind the parent guard
// (#6638); the stub answers the held variants like the probes.
func (s *stubReEnrollAuthorizer) HoldAccountStudentsPermission(ctx context.Context, accountID int64, studentIDs, _ []int64, tenantID int64, permission string) (map[int64]bool, error) {
	grants := map[int64]bool{}
	for _, id := range studentIDs {
		granted, err := s.AccountHasStudentPermission(ctx, accountID, id, tenantID, permission)
		if err != nil {
			return nil, err
		}
		grants[id] = granted
	}
	return grants, nil
}

func (s *stubReEnrollAuthorizer) HoldGuardianEmailStudentsPermission(ctx context.Context, email string, studentIDs, _ []int64, tenantID int64, permission string) (map[int64]bool, error) {
	grants := map[int64]bool{}
	for _, id := range studentIDs {
		granted, err := s.GuardianEmailHasStudentPermission(ctx, email, id, tenantID, permission)
		if err != nil {
			return nil, err
		}
		grants[id] = granted
	}
	return grants, nil
}

type matchedStudentFixture struct {
	svc      *ChangeRequests
	children *stubMatchedStudentChildRepo
	requests *stubMatchLockRequestRepo
	auth     *stubReEnrollAuthorizer
	req      *enrollmentNative.Request
	phase    *enrollmentNative.Phase
	row      *ChangeRequest
}

func newMatchedStudentFixture(audience string, resolved *int64, collision bool) *matchedStudentFixture {
	children := &stubMatchedStudentChildRepo{}
	requests := &stubMatchLockRequestRepo{has: collision}
	auth := &stubReEnrollAuthorizer{granted: true}
	svc := NewChangeRequests(ChangeRequestDependencies{Settings: intakeSettingsFake{},
		Requests:     requests,
		Children:     children,
		Students:     &stubMatchStudentRepo{matchID: resolved, exists: resolved != nil},
		GuardianHold: auth,
		ParentsURL:   "https://parents.example.test",
	})

	accountID := int64(4242)
	req := &enrollmentNative.Request{GuardianAccountID: &accountID}
	req.TenantID = 77

	return &matchedStudentFixture{
		svc:      svc,
		children: children,
		requests: requests,
		auth:     auth,
		req:      req,
		phase:    &enrollmentNative.Phase{Audience: audience},
		row:      &ChangeRequest{},
	}
}

// reconcile runs the approval's pin reconciliation for the fixture's request,
// phase and change request: no capacity overrides, eligibility enforced,
// child index 0.
func (f *matchedStudentFixture) reconcile(child *enrollmentNative.RequestChild, next SubmitChild) error {
	run := &approval{
		row: f.row, req: f.req, children: []*enrollmentNative.RequestChild{child},
		prepared:  &proposal{phase: f.phase, request: SubmitRequest{Children: []SubmitChild{next}}},
		overrides: map[int]string{}, eligibilityEnforced: true,
	}
	if err := f.svc.holdApprovalAuthority(context.Background(), run); err != nil {
		return err
	}
	return f.svc.reconcileMatchedStudent(context.Background(), run, child, next, 0)
}

func matchedStudentChild(id int64, pin *int64) *enrollmentNative.RequestChild {
	child := &enrollmentNative.RequestChild{
		FirstName:        "Anna",
		LastName:         "Müller",
		DateOfBirth:      "2019-04-12",
		Status:           enrollmentNative.ChildStatusUnderReview,
		MatchedStudentID: pin,
	}
	child.ID = id
	return child
}

func matchedStudentSubmit(firstName string) SubmitChild {
	return SubmitChild{
		FirstName:   firstName,
		LastName:    "Müller",
		DateOfBirth: calendar.NewDate(2019, 4, 12),
	}
}

// An approved change request may rewrite name + birthday. The submission-time
// pin then describes a different human than the reviewed row, so it must be
// re-resolved against the new identity before the decision service renews
// anything (#1663).
func TestReconcileMatchedStudent_RepinsAfterIdentityEdit(t *testing.T) {
	t.Parallel()

	newStudent := int64(900)
	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, &newStudent, false)
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
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
	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, &pin, false)
	child := matchedStudentChild(11, &pin)

	err := f.reconcile(child, matchedStudentSubmit("Anna"))
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

	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceOpen, nil, false)
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
	require.NoError(t, err)

	require.Len(t, f.children.writes, 1)
	assert.Nil(t, f.children.writes[0], "the stale pin must be cleared")
	assert.Nil(t, child.MatchedStudentID)
}

// On an existing_students phase an edit to a child that matches no enrolled
// student is not a fresh create — it is an invalid submission for the phase.
func TestReconcileMatchedStudent_RejectsUnmatchableIdentityOnExistingStudentsPhase(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, nil, false)
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
	require.ErrorIs(t, err, enrollmentNative.ErrChildNotEnrolled)
	assert.Empty(t, f.children.writes, "a rejected reconcile must not persist anything")
}

// The re-resolved pin is only allowed to stand when the guardian holds
// parent_portal.enrollmentNative.submit on THAT student.
func TestReconcileMatchedStudent_RejectsUnpermittedNewMatch(t *testing.T) {
	t.Parallel()

	newStudent := int64(900)
	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, &newStudent, false)
	f.auth.granted = false
	oldPin := int64(100)
	child := matchedStudentChild(11, &oldPin)

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
	require.ErrorIs(t, err, enrollmentNative.ErrChildEnrollmentNotPermitted)
	assert.Empty(t, f.children.writes)
}

// HasActiveRequestForMatchedStudent ignores rejected rows, so another guardian
// may legitimately pin the same student while this request sits rejected.
// Reopening it to under_review puts both in competition again — the uniqueness
// guard must therefore run at reopen time, not only at insert time (#1663).
func TestReconcileMatchedStudent_RejectsReopenCollidingWithAnotherRequest(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, &pin, true)
	child := matchedStudentChild(11, &pin)
	child.Status = enrollmentNative.ChildStatusRejected
	f.row.BaseSnapshot = map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna"}}}
	f.row.ProposedSnapshot = map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna-Lena"}}}

	err := f.reconcile(child, matchedStudentSubmit("Anna"))
	require.ErrorIs(t, err, enrollmentNative.ErrExistingStudentAlreadyRequested)
}

// A rejected child whose own data did NOT change stays rejected, so it never
// re-enters competition and must not be blocked by another request's pin.
func TestReconcileMatchedStudent_SkipsUniquenessForChildStayingRejected(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, &pin, true)
	child := matchedStudentChild(11, &pin)
	child.Status = enrollmentNative.ChildStatusRejected

	err := f.reconcile(child, matchedStudentSubmit("Anna"))
	require.NoError(t, err)
	assert.Empty(t, f.requests.hasStudentIDs, "a child staying rejected must not be probed")
}

// Once approval materialized the student, SyncApprovedChildData owns that live
// record and the pin is history — reconciliation must not touch it.
func TestReconcileMatchedStudent_SkipsMaterializedChild(t *testing.T) {
	t.Parallel()

	pin := int64(100)
	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, &pin, true)
	child := matchedStudentChild(11, &pin)
	student := int64(555)
	child.CreatedStudentID = &student

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
	require.NoError(t, err)
	assert.Empty(t, f.children.writes)
	assert.Empty(t, f.requests.hasStudentIDs)
}

// Rollover children resolve their existing student through the rollover source
// chain, which takes precedence at approval — a pin here would be dead data.
func TestReconcileMatchedStudent_SkipsRolloverChild(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceExistingStudents, nil, false)
	child := matchedStudentChild(11, nil)
	source := int64(9)
	child.RolloverSourceChildID = &source

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
	require.NoError(t, err)
	assert.Empty(t, f.children.writes)
}

// An unpinned child on an ordinary phase is the common case and must stay a
// pure no-op — no lookups, no probes, no writes.
func TestReconcileMatchedStudent_NoOpForUnpinnedOrdinaryChild(t *testing.T) {
	t.Parallel()

	f := newMatchedStudentFixture(enrollmentNative.PhaseAudienceOpen, nil, true)
	child := matchedStudentChild(11, nil)

	err := f.reconcile(child, matchedStudentSubmit("Ben"))
	require.NoError(t, err)
	assert.Empty(t, f.children.writes)
	assert.Empty(t, f.requests.hasStudentIDs)
}

// postApprovalChildStatus has to mirror the write loop's status decisions,
// because the uniqueness guard keys on whether the child ends up active.
func TestPostApprovalChildStatus(t *testing.T) {
	t.Parallel()

	row := &ChangeRequest{
		BaseSnapshot:     map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna"}}},
		ProposedSnapshot: map[string]any{"children": []any{map[string]any{"id": int64(11), "first_name": "Anna-Lena"}}},
	}
	child := matchedStudentChild(11, nil)

	t.Run("capacity override wins", func(t *testing.T) {
		child.Status = enrollmentNative.ChildStatusUnderReview
		assert.Equal(t, enrollmentNative.ChildStatusWaitlisted,
			postApprovalChildStatus(row, child, map[int]string{0: enrollmentNative.ChildStatusWaitlisted}, 0))
	})

	t.Run("rejected child with changed data reopens", func(t *testing.T) {
		child.Status = enrollmentNative.ChildStatusRejected
		assert.Equal(t, enrollmentNative.ChildStatusUnderReview,
			postApprovalChildStatus(row, child, map[int]string{}, 0))
	})

	t.Run("rejected child without own change stays rejected", func(t *testing.T) {
		child.Status = enrollmentNative.ChildStatusRejected
		unchanged := &ChangeRequest{
			BaseSnapshot:     row.BaseSnapshot,
			ProposedSnapshot: row.BaseSnapshot,
		}
		assert.Equal(t, enrollmentNative.ChildStatusRejected,
			postApprovalChildStatus(unchanged, child, map[int]string{}, 0))
	})

	t.Run("otherwise unchanged", func(t *testing.T) {
		child.Status = enrollmentNative.ChildStatusUnderReview
		assert.Equal(t, enrollmentNative.ChildStatusUnderReview,
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

	phase := &enrollmentNative.Phase{
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
		require.ErrorIs(t, err, enrollmentNative.ErrChildClassNotEligible)
	})

	t.Run("missing class rejected when the phase restricts", func(t *testing.T) {
		err := validateChildClassEligibility(phase, []SubmitChild{{FirstName: "Kim"}})
		require.ErrorIs(t, err, enrollmentNative.ErrChildClassNotEligible)
	})

	t.Run("unrestricted phase accepts anything", func(t *testing.T) {
		require.NoError(t, validateChildClassEligibility(&enrollmentNative.Phase{},
			[]SubmitChild{{FirstName: "Kim", TargetSchoolClass: classPtr("9z")}}))
	})
}

// Reusing one transaction's held grants must preserve refusals and distinguish
// the school, identity and student. A standalone sync still takes a fresh hold.
func TestApprovedChildSyncUsesOnlyItsHeldAuthority(t *testing.T) {
	t.Parallel()
	accountID, studentID := int64(42), int64(70)
	request := &enrollmentNative.Request{GuardianAccountID: &accountID}
	request.TenantID = 90
	key := approvalAuthorityKey{tenantID: request.TenantID, submitter: "account:42"}
	for _, scenario := range []string{"granted", "refused", "other school", "other account", "other student", "standalone"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			identity := *request
			student := studentID
			holds := renewalHolds{studentID: scenario != "refused"}
			ctx := context.WithValue(context.Background(), approvalAuthorityReadsKey{}, &approvalAuthorityReads{
				holds: map[approvalAuthorityKey]renewalHolds{key: holds},
			})
			switch scenario {
			case "other school":
				identity.TenantID++
			case "other account":
				other := accountID + 1
				identity.GuardianAccountID = &other
			case "other student":
				student++
			case "standalone":
				ctx = context.Background()
			}
			hold := &stubReEnrollAuthorizer{granted: true}
			decisions := &Decisions{deps: DecisionDependencies{GuardianHold: hold}}
			run := &approvedChildSync{request: &identity, child: &enrollmentNative.RequestChild{CreatedStudentID: &student}}
			lapsed, err := decisions.childEnrollmentOnly(ctx, run)
			require.NoError(t, err)
			require.Equal(t, scenario != "granted" && scenario != "standalone", lapsed)
			if scenario == "standalone" {
				require.Equal(t, []int64{studentID}, hold.gotStudents)
			} else {
				require.Empty(t, hold.gotStudents, "an approval must not acquire more authority after the phase locks")
			}
		})
	}
}
