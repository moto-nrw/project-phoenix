package enrollment_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type failingIntakeWrites struct {
	enrollmentService.IntakeRequests
	enrollmentService.IntakeChildren
	enrollmentService.IntakeGuardians
	enrollmentService.IntakeLateInvites
	writes    int
	failAfter int
	failure   error
}

type failingSubmissionRateLimiter struct {
	enrollmentService.SubmissionRateLimiter
	failKey string
	failure error
}

func (r *failingSubmissionRateLimiter) IncrementAttempts(ctx context.Context, tenantID int64, keyType, key string, window time.Duration) (*capability.SubmissionRateLimitState, error) {
	if keyType == r.failKey {
		return nil, r.failure
	}
	return r.SubmissionRateLimiter.IncrementAttempts(ctx, tenantID, keyType, key, window)
}

func TestPublicIntakeRejectsRateLimitStorageFailures(t *testing.T) {
	t.Parallel()
	for _, keyType := range []string{capability.SubmissionRateLimitKeyTypeIP, capability.SubmissionRateLimitKeyTypeEmail} {
		t.Run(keyType, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			env, cleanup := setupRequestTest(t)
			defer cleanup()
			failure := errors.New("injected throttle storage failure")
			limiter := &failingSubmissionRateLimiter{SubmissionRateLimiter: env.config.RateLimitRepo, failKey: keyType, failure: failure}
			config := env.config
			config.RateLimitRepo = limiter
			service := enrollmentService.NewRequestService(config)
			input := validSubmission(t, env.phaseID)
			input.RemoteIP = "198.51.100.42"
			input.SuppressSubmissionEmails = true
			input.AdditionalGuardians = []enrollmentService.SubmitGuardian{{FirstName: "Second", LastName: "Guardian"}}
			result, err := service.Submit(testpkg.Ctx(t), input)
			require.ErrorIs(t, err, failure)
			require.Nil(t, result)
			assertIntakeCounts(t, env.db, env.phaseID, 0, 0)
			var buckets int
			require.NoError(t, env.db.NewSelect().TableExpr("enrollment.submission_rate_limits").Where("tenant_id = ?", testpkg.Tenant(t)).ColumnExpr("count(*)").Scan(testpkg.Ctx(t), &buckets))
			require.Zero(t, buckets, "an email failure must roll back the preceding IP increment")
			limiter.failKey = ""
			result, err = service.Submit(testpkg.Ctx(t), input)
			require.NoError(t, err)
			require.NotNil(t, result)
			assertIntakeCounts(t, env.db, env.phaseID, 1, 1)
		})
	}
}

func (w *failingIntakeWrites) afterWrite() error {
	w.writes++
	if w.writes == w.failAfter {
		return w.failure
	}
	return nil
}

func (w *failingIntakeWrites) InsertRequest(ctx context.Context, request *capability.Request) error {
	if err := w.IntakeRequests.InsertRequest(ctx, request); err != nil {
		return err
	}
	return w.afterWrite()
}

func (w *failingIntakeWrites) InsertChild(ctx context.Context, child *capability.RequestChild) error {
	if err := w.IntakeChildren.InsertChild(ctx, child); err != nil {
		return err
	}
	return w.afterWrite()
}

func (w *failingIntakeWrites) CreateRequestGuardian(ctx context.Context, guardian *capability.RequestGuardian) error {
	if err := w.IntakeGuardians.CreateRequestGuardian(ctx, guardian); err != nil {
		return err
	}
	return w.afterWrite()
}

func (w *failingIntakeWrites) MarkLateInviteUsed(ctx context.Context, inviteID, requestID int64, at time.Time) error {
	if err := w.IntakeLateInvites.MarkLateInviteUsed(ctx, inviteID, requestID, at); err != nil {
		return err
	}
	return w.afterWrite()
}

func TestPublicIntakeRollsBackAfterEveryRequestInviteGuardianAndChildWrite(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		lateInvite bool
		failAfter  int
	}{
		{false, 1}, {false, 2}, {false, 3}, {false, 4},
		{true, 1}, {true, 2}, {true, 3}, {true, 4}, {true, 5},
	} {
		failAfter := scenario.failAfter
		t.Run(fmt.Sprintf("late_invite_%t_write_%d", scenario.lateInvite, failAfter), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			env, cleanup := setupRequestTest(t)
			defer cleanup()
			env.settings.stringValues[configModel.KeyEnrollmentDuplicateHandling] = configModel.EnrollmentDuplicateHandlingBlock
			failure := errors.New("injected failure after intake write")
			writes := &failingIntakeWrites{
				IntakeRequests: env.config.Requests, IntakeChildren: env.config.Children,
				IntakeGuardians:   env.config.Guardians,
				IntakeLateInvites: enrollmentTest.New(),
				failAfter:         failAfter, failure: failure,
			}
			config := env.config
			config.Requests = writes
			config.Children = writes
			config.Guardians = writes
			config.LateInviteRepo = writes
			service := enrollmentService.NewRequestService(config)
			input := validSubmission(t, env.phaseID)
			input.SuppressSubmissionEmails = true
			input.AdditionalGuardians = []enrollmentService.SubmitGuardian{{FirstName: "Second", LastName: "Guardian"}}
			second := input.Children[0]
			second.FirstName = "Second"
			input.Children = append(input.Children, second)
			ctx := testpkg.Ctx(t)
			var created *enrollmentService.CreateLateInviteResult
			if scenario.lateInvite {
				var err error
				created, err = service.CreateLateInvite(ctx, enrollmentService.CreateLateInviteInput{PhaseID: env.phaseID, GuardianEmail: input.GuardianEmail, CreatedBy: env.creatorID})
				require.NoError(t, err)
				input.LateInviteToken = created.Token
			}
			result, err := service.Submit(ctx, input)
			require.ErrorIs(t, err, failure)
			require.Nil(t, result)
			require.Equal(t, failAfter, writes.writes)
			assertIntakeCounts(t, env.db, env.phaseID, 0, 0)
			if created != nil {
				unused, err := writes.UsableLateInvite(ctx, created.Invite.TokenHash, env.phaseID, time.Now(), false)
				require.NoError(t, err)
				require.Nil(t, unused.UsedAt)
				require.Nil(t, unused.UsedRequestID)
			}

			writes.failAfter = 0
			result, err = service.Submit(ctx, input)
			require.NoError(t, err)
			require.Len(t, result.Children, 2)
			assertIntakeCounts(t, env.db, env.phaseID, 1, 2)
			duplicate, err := service.Submit(ctx, input)
			if created == nil {
				require.ErrorIs(t, err, enrollmentService.ErrDuplicateEnrollment)
			} else {
				require.ErrorIs(t, err, enrollmentService.ErrLateInviteInvalid)
				used, lookupErr := writes.LateInviteByUsedRequestID(ctx, result.Request.ID)
				require.NoError(t, lookupErr)
				require.Equal(t, created.Invite.ID, used.ID)
			}
			require.Nil(t, duplicate)
			assertIntakeCounts(t, env.db, env.phaseID, 1, 2)
		})
	}
}

func assertIntakeCounts(t *testing.T, db *bun.DB, phaseID int64, requests, children int) {
	t.Helper()
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var requestCount, childCount, guardianCount int
		require.NoError(t, tx.NewRaw("SELECT COUNT(*) FROM enrollment.requests WHERE phase_id = ?", phaseID).Scan(ctx, &requestCount))
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM enrollment.request_children c
			JOIN enrollment.requests r ON r.id = c.request_id WHERE r.phase_id = ?`, phaseID).Scan(ctx, &childCount))
		require.Equal(t, requests, requestCount)
		require.Equal(t, children, childCount)
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM enrollment.request_guardians g
			JOIN enrollment.requests r ON r.id = g.request_id WHERE r.phase_id = ?`, phaseID).Scan(ctx, &guardianCount))
		require.Equal(t, requests, guardianCount, "each successful request has exactly one additional guardian")
		return nil
	})
	require.NoError(t, err)
}

type failingIntakeOutbox struct {
	delivery  testEnrollmentDelivery
	tenantID  int64
	writes    int
	failAfter int
	requestID int64
	failure   error
}

func (o *failingIntakeOutbox) EnqueueOutbox(ctx context.Context, request platformModels.OutboxEnqueueRequest) error {
	payload, err := json.Marshal(request.Payload)
	if err != nil {
		return err
	}
	o.requestID = request.RelatedEntityID
	key := fmt.Sprintf("intake-rollback-%d-%d", request.RelatedEntityID, o.writes)
	if _, err := enqueueTestEnrollmentEmailInContext(ctx, o.delivery, o.tenantID, request.RelatedEntityID, key, payload); err != nil {
		return err
	}
	o.writes++
	if o.writes == o.failAfter {
		return o.failure
	}
	return nil
}

func TestPublicIntakeRollsBackAfterEachSubmissionEmailEnqueue(t *testing.T) {
	t.Parallel()
	for _, failAfter := range []int{1, 2} {
		t.Run(fmt.Sprintf("email_%d", failAfter), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			env, cleanup := setupRequestTest(t)
			defer cleanup()
			env.settings.stringValues[configModel.KeyEnrollmentNotificationEmails] = "admin@example.test"
			env.settings.stringValues[configModel.KeyEnrollmentDuplicateHandling] = configModel.EnrollmentDuplicateHandlingBlock
			failure := errors.New("injected after persisted submission email")
			outbox := &failingIntakeOutbox{delivery: newTestEnrollmentDelivery(t, env.db), tenantID: testpkg.Tenant(t), failAfter: failAfter, failure: failure}
			config := env.config
			config.OutboxEnqueuer = outbox
			service := enrollmentService.NewRequestService(config)
			input := validSubmission(t, env.phaseID)
			input.AdditionalGuardians = []enrollmentService.SubmitGuardian{{FirstName: "Second", LastName: "Guardian"}}
			ctx := testpkg.Ctx(t)
			result, err := service.Submit(ctx, input)
			require.ErrorIs(t, err, failure)
			require.Nil(t, result)
			require.Equal(t, failAfter, outbox.writes)
			assertIntakeCounts(t, env.db, env.phaseID, 0, 0)
			assertSubmissionEmailCount(t, env.db, outbox.delivery, outbox.requestID, 0)
			outbox.failAfter = 0
			result, err = service.Submit(ctx, input)
			require.NoError(t, err)
			require.Len(t, result.Children, 1)
			assertIntakeCounts(t, env.db, env.phaseID, 1, 1)
			assertSubmissionEmailCount(t, env.db, outbox.delivery, result.Request.ID, 2)
			duplicate, err := service.Submit(ctx, input)
			require.ErrorIs(t, err, enrollmentService.ErrDuplicateEnrollment)
			require.Nil(t, duplicate)
			assertSubmissionEmailCount(t, env.db, outbox.delivery, result.Request.ID, 2)
		})
	}
}

func assertSubmissionEmailCount(t *testing.T, db *bun.DB, delivery testEnrollmentDelivery, requestID int64, expected int) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		count, err := delivery.CountRelatedEmails(ctx, "enrollment_request", requestID)
		require.NoError(t, err)
		require.Equal(t, expected, count)
		return nil
	}))
}

type failingIntakeOfferingLinks struct {
	enrollmentService.IntakeChildren
	writes    int
	failAfter int
	failure   error
}

func (r *failingIntakeOfferingLinks) InsertRequestChildOffering(ctx context.Context, link *capability.RequestChildOffering) error {
	if err := r.IntakeChildren.InsertRequestChildOffering(ctx, link); err != nil {
		return err
	}
	r.writes++
	if r.writes == r.failAfter {
		return r.failure
	}
	return nil
}

func TestPublicIntakeRollsBackAfterEachOfferingLinkWrite(t *testing.T) {
	t.Parallel()
	for _, failAfter := range []int{1, 2} {
		t.Run(fmt.Sprintf("link_%d", failAfter), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			env, cleanup := setupRequestTest(t)
			defer cleanup()
			env.settings.stringValues[configModel.KeyEnrollmentDuplicateHandling] = configModel.EnrollmentDuplicateHandlingBlock
			ctx := testpkg.Ctx(t)
			var offeringIDs []int64
			for _, name := range []string{"First", "Second"} {
				offering := &enrollmentModels.CareOffering{PhaseID: env.phaseID, Name: name, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}, IsActive: true}
				require.NoError(t, env.config.CareOfferingRepo.Create(ctx, offering))
				offeringIDs = append(offeringIDs, offering.ID)
			}
			failure := errors.New("injected after offering link write")
			links := &failingIntakeOfferingLinks{IntakeChildren: env.config.Children, failAfter: failAfter, failure: failure}
			config := env.config
			config.Children = links
			service := enrollmentService.NewRequestService(config)
			input := validSubmission(t, env.phaseID)
			input.SuppressSubmissionEmails = true
			input.AdditionalGuardians = []enrollmentService.SubmitGuardian{{FirstName: "Second", LastName: "Guardian"}}
			input.Children[0].OfferingIDs = offeringIDs
			result, err := service.Submit(ctx, input)
			require.ErrorIs(t, err, failure)
			require.Nil(t, result)
			require.Equal(t, failAfter, links.writes)
			assertIntakeCounts(t, env.db, env.phaseID, 0, 0)
			assertIntakeOfferingLinkCount(t, env.db, offeringIDs, 0)
			links.failAfter = 0
			result, err = service.Submit(ctx, input)
			require.NoError(t, err)
			require.Len(t, result.Children, 1)
			assertIntakeCounts(t, env.db, env.phaseID, 1, 1)
			assertIntakeOfferingLinkCount(t, env.db, offeringIDs, 2)
			duplicate, err := service.Submit(ctx, input)
			require.ErrorIs(t, err, enrollmentService.ErrDuplicateEnrollment)
			require.Nil(t, duplicate)
			assertIntakeOfferingLinkCount(t, env.db, offeringIDs, 2)
		})
	}
}

func assertIntakeOfferingLinkCount(t *testing.T, db *bun.DB, offeringIDs []int64, expected int) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var count int
		require.NoError(t, tx.NewRaw("SELECT COUNT(*) FROM enrollment.request_child_offerings WHERE care_offering_id IN (?)", bun.List(offeringIDs)).Scan(ctx, &count))
		require.Equal(t, expected, count)
		return nil
	}))
}
