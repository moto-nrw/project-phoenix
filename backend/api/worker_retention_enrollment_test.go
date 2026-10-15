package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// enrollmentFixture writes the enrollments of one school through the
// Enrollment owner and backdates them as the superuser.
type enrollmentFixture struct {
	suite    retentionSuite
	tenantID int64
	ctx      context.Context
	owner    *enrollment.Module
	phaseID  int64
	creator  int64
}

func (s retentionSuite) enrollments(t *testing.T, tenantID int64) enrollmentFixture {
	t.Helper()
	ctx := testpkg.WithTenantRuntime(t, testpkg.TenantContext(tenantID), s.admin)
	owner := enrollmentCompose.New()
	phase := &enrollment.Phase{
		Name:                      fmt.Sprintf("Löschfrist-%d", tenantID),
		Kind:                      enrollment.PhaseKindSchoolYear,
		ServiceStartDate:          enrollment.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:            enrollment.Date(calendar.NewDate(2027, 7, 31)),
		CareOverflowMode:          enrollment.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollment.PhaseCareOfferingSelectionOptional,
		IsActive:                  true,
	}
	require.NoError(t, owner.InsertPhase(ctx, phase))
	creator := testpkg.CreateTestAccount(t, s.admin, fmt.Sprintf("late-invite-%d@example.com", tenantID)).ID
	return enrollmentFixture{suite: s, tenantID: tenantID, ctx: ctx, owner: owner, phaseID: phase.ID, creator: creator}
}

// settledChild is one child of a request: its status, how many days back it
// was settled and whether a child record was created from it. A lapsed child
// was withdrawn by the rollover deadline, which stamps no review.
type settledChild struct {
	status  string
	daysAgo int
	lapsed  bool
	student int64
}

// request stores a request with the children and returns its ID.
func (f enrollmentFixture) request(t *testing.T, label string, children ...settledChild) int64 {
	t.Helper()
	request := &enrollment.Request{
		PhaseID:           f.phaseID,
		GuardianFirstName: "Erledigt",
		GuardianLastName:  label,
		GuardianEmail:     fmt.Sprintf("%s-%d@example.invalid", label, f.tenantID),
		ConsentFlags:      []byte("{}"),
		CustomData:        []byte("{}"),
		SubmissionSource:  enrollment.RequestSourcePublic,
		SourceMetadata:    []byte("{}"),
		StatusToken:       fmt.Sprintf("%s-token-%d", label, f.tenantID),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, f.owner.InsertRequest(f.ctx, request))
	for i, settled := range children {
		child := &enrollment.RequestChild{
			RequestID:      request.ID,
			FirstName:      fmt.Sprintf("%s-%d", label, i),
			LastName:       "Kind",
			DateOfBirth:    enrollment.Date(calendar.NewDate(2019, 4, 15)),
			CustomData:     []byte("{}"),
			Status:         settled.status,
			ActivationMode: enrollment.ChildActivationScheduled,
			SortOrder:      i,
		}
		if settled.student > 0 {
			child.CreatedStudentID = &settled.student
		}
		require.NoError(t, f.owner.InsertChild(f.ctx, child))
		_, err := f.suite.admin.ExecContext(context.Background(), `UPDATE enrollment.request_children
			SET reviewed_at = CASE WHEN ? THEN NULL ELSE CURRENT_TIMESTAMP - make_interval(days => ?) END,
				updated_at = CURRENT_TIMESTAMP - make_interval(days => ?)
			WHERE id = ?`, settled.lapsed, settled.daysAgo, settled.daysAgo, child.ID)
		require.NoError(t, err)
	}
	return request.ID
}

// lateInvite stores an invitation to enroll late that expired daysAgo days
// back, used by usedBy when that is set.
func (f enrollmentFixture) lateInvite(t *testing.T, label string, daysAgo int, usedBy int64) int64 {
	t.Helper()
	invite := &enrollment.LateInvite{
		PhaseID: f.phaseID, TokenHash: fmt.Sprintf("%s-hash-%d", label, f.tenantID),
		GuardianEmail: fmt.Sprintf("%s-%d@example.invalid", label, f.tenantID), ExpiresAt: time.Now().Add(time.Hour), CreatedBy: f.creator,
	}
	require.NoError(t, f.owner.InsertLateInvite(f.ctx, invite))
	_, err := f.suite.admin.ExecContext(context.Background(), `UPDATE enrollment.late_invites
		SET created_at = CURRENT_TIMESTAMP - make_interval(days => ? + 30),
			expires_at = CURRENT_TIMESTAMP - make_interval(days => ?),
			used_request_id = NULLIF(?, 0), used_at = CASE WHEN ? > 0 THEN CURRENT_TIMESTAMP - make_interval(days => ?) END
		WHERE id = ?`, daysAgo, daysAgo, usedBy, usedBy, daysAgo+1, invite.ID)
	require.NoError(t, err)
	return invite.ID
}

// mail queues an enrollment mail of the request.
func (f enrollmentFixture) mail(t *testing.T, requestID int64) int64 {
	t.Helper()
	module, err := NewEnrollmentFlowDelivery(testpkg.MustDatabase(f.suite.admin))
	require.NoError(t, err)
	var queued delivery.Enqueued
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), f.suite.admin, f.tenantID, func(ctx context.Context, _ bun.Tx) error {
		queued, err = module.EnqueueEmail(ctx, delivery.EmailIntent{
			TenantID: f.tenantID, Template: "enrollment_test", Recipient: delivery.EmailRecipient{Address: "guardian@example.invalid"},
			Payload: []byte("{}"), IdempotencyKey: fmt.Sprintf("settled-%d", requestID),
			Related: delivery.RelatedEntity{Type: "enrollment_request", ID: requestID},
		})
		return err
	}))
	return queued.ID
}

func TestNightlyRetentionDeletesSettledEnrollmentsWithoutAChild(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)
	enrolled := testpkg.CreateTestStudent(t, suite.admin, "Angenommen", "Kind", "1a").ID
	f := suite.enrollments(t, school)
	other := suite.enrollments(t, otherSchool)
	withdrawn, rejected := enrollment.ChildStatusWithdrawn, enrollment.ChildStatusRejected

	expiredWithdrawn := f.request(t, "zurueckgezogen", settledChild{status: withdrawn, daysAgo: 91})
	expiredLapsed := f.request(t, "verfallen", settledChild{status: withdrawn, daysAgo: 100, lapsed: true})
	expiredMixed := f.request(t, "gemischt", settledChild{status: withdrawn, daysAgo: 120}, settledChild{status: rejected, daysAgo: 91})
	keptYoung := f.request(t, "jung", settledChild{status: withdrawn, daysAgo: 89})
	keptRejectedWindow := f.request(t, "abgelehnt-jung", settledChild{status: withdrawn, daysAgo: 200}, settledChild{status: rejected, daysAgo: 30})
	keptFullyRejected := f.request(t, "abgelehnt", settledChild{status: rejected, daysAgo: 200})
	keptApproved := f.request(t, "angenommen",
		settledChild{status: withdrawn, daysAgo: 200}, settledChild{status: enrollment.ChildStatusApproved, daysAgo: 200, student: enrolled})
	keptOpen := f.request(t, "offen", settledChild{status: enrollment.ChildStatusSubmitted, daysAgo: 200})
	keptWaitlisted := f.request(t, "warteliste", settledChild{status: enrollment.ChildStatusWaitlisted, daysAgo: 200})
	otherExpired := other.request(t, "andere-schule", settledChild{status: withdrawn, daysAgo: 200})

	queued := f.mail(t, expiredWithdrawn)
	usedInvite := f.lateInvite(t, "eingeloest", 120, expiredLapsed)
	expiredInvite := f.lateInvite(t, "verfallen", 91, 0)
	keptInvite := f.lateInvite(t, "kuerzlich", 30, 0)
	otherInvite := other.lateInvite(t, "andere-schule", 200, 0)

	suite.run(t, school)
	suite.run(t, school)

	for _, id := range []int64{expiredWithdrawn, expiredLapsed, expiredMixed} {
		assert.False(t, suite.exists(t, "enrollment.requests", id), "a settled request without a child goes after its window")
		assert.Equal(t, 1, suite.count(t, `SELECT COUNT(*) FROM audit.enrollment_deletions
			WHERE tenant_id = ? AND request_id = ? AND actor_type = 'system'`, school, id), "each deleted request keeps its deletion record")
	}
	assert.True(t, suite.exists(t, "enrollment.requests", keptYoung), "a request settled 89 days back stays")
	assert.True(t, suite.exists(t, "enrollment.requests", keptRejectedWindow), "a rejected child keeps the window of rejected enrollments")
	assert.True(t, suite.exists(t, "enrollment.requests", keptFullyRejected), "fully rejected requests keep their own cleanup")
	assert.True(t, suite.exists(t, "enrollment.requests", keptApproved), "an accepted enrollment goes with its child")
	assert.True(t, suite.exists(t, "enrollment.requests", keptOpen), "an open enrollment stays")
	assert.True(t, suite.exists(t, "enrollment.requests", keptWaitlisted), "a waitlisted enrollment stays")
	assert.True(t, suite.exists(t, "enrollment.requests", otherExpired), "the run of one school never touches another")

	assert.Equal(t, 1, suite.count(t, `SELECT COUNT(*) FROM platform.email_outbox WHERE id = ? AND status = 'cancelled'`, queued),
		"the queued mail of a deleted request is cancelled")
	assert.False(t, suite.exists(t, "enrollment.late_invites", usedInvite), "the invitation a deleted request used goes with it")
	assert.False(t, suite.exists(t, "enrollment.late_invites", expiredInvite), "an unused invitation goes once its window after expiry passed")
	assert.True(t, suite.exists(t, "enrollment.late_invites", keptInvite))
	assert.True(t, suite.exists(t, "enrollment.late_invites", otherInvite))

	assert.Equal(t, []int{5}, suite.loggedDeletions(t, school, retentionSettledEnrollments),
		"one entry counting three requests and two invitations; the second run deletes nothing")
	assert.Empty(t, suite.loggedDeletions(t, otherSchool, retentionSettledEnrollments))
}

func TestNightlyRetentionFollowsTheSchoolsRecordedSettledEnrollmentWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.withdrawn_enrollment_retention_days', '200'::jsonb)`, school)
	require.NoError(t, err)
	f := suite.enrollments(t, school)

	kept := f.request(t, "lange-frist", settledChild{status: enrollment.ChildStatusWithdrawn, daysAgo: 150})
	expired := f.request(t, "abgelaufen", settledChild{status: enrollment.ChildStatusWithdrawn, daysAgo: 201})

	suite.run(t, school)

	assert.True(t, suite.exists(t, "enrollment.requests", kept), "a longer contractual window keeps the request")
	assert.False(t, suite.exists(t, "enrollment.requests", expired))
	assert.Equal(t, []int{1}, suite.loggedDeletions(t, school, retentionSettledEnrollments))
}
