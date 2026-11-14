package enrollmenthttp_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The limits of the public submission count stored submissions only (#6726):
// families behind one address pass, a refused attempt takes nothing from a
// budget, and parallel submissions cannot pass a limit together.

const (
	storedSubmissionsPerIP    = 100
	storedSubmissionsPerEmail = 10
)

// fillSubmissionBucket puts a running bucket of the test's school at count.
func fillSubmissionBucket(t *testing.T, db *bun.DB, keyType, key string, count int) {
	t.Helper()
	_, err := db.NewRaw(`INSERT INTO enrollment.submission_rate_limits (tenant_id, key_type, key_value, attempts, window_start)
		VALUES (?, ?, ?, ?, NOW())`, testpkg.Tenant(t), keyType, key, count).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

// submissionBucket is the count of a bucket of the test's school, 0 without
// one.
func submissionBucket(t *testing.T, db *bun.DB, keyType, key string) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT COALESCE(SUM(attempts), 0) FROM enrollment.submission_rate_limits
		WHERE tenant_id = ? AND key_type = ? AND key_value = ?`, testpkg.Tenant(t), keyType, key).Scan(testpkg.Ctx(t), &count))
	return count
}

// familySubmission is a valid submission of family number i from remoteIP.
func familySubmission(t *testing.T, phaseID int64, remoteIP string, i int) enrollmentAPI.SubmitRequest {
	t.Helper()
	req := validSubmission(t, phaseID)
	req.RemoteIP = remoteIP
	req.GuardianEmail = "familie" + strconv.Itoa(i) + "@example.com"
	req.Children[0].FirstName = "Kind" + strconv.Itoa(i)
	req.Children[0].LastName = "Familie" + strconv.Itoa(i)
	return req
}

func TestSubmitLetsFamiliesBehindOneAddressThrough(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	const schoolNetwork = "203.0.113.20"

	// Twelve families of one school network: the old limit of ten attempts
	// an hour refused the eleventh.
	for i := range 12 {
		_, err := env.svc.Submit(testpkg.Ctx(t), familySubmission(t, env.phaseID, schoolNetwork, i))
		require.NoError(t, err, "family %d of the same network", i+1)
	}
	assert.Equal(t, 12, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, schoolNetwork))
	assertIntakeRequests(t, env, 12)
}

func TestSubmitRefusesPastTheStoredLimitOfAnAddress(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	const address = "203.0.113.21"
	fillSubmissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address, storedSubmissionsPerIP-1)

	_, err := env.svc.Submit(testpkg.Ctx(t), familySubmission(t, env.phaseID, address, 1))
	require.NoError(t, err, "the hundredth stored submission of the hour passes")

	over := familySubmission(t, env.phaseID, address, 2)
	_, err = env.svc.Submit(testpkg.Ctx(t), over)
	require.ErrorIs(t, err, enrollmentNative.ErrRateLimited)
	require.NotErrorIs(t, err, enrollmentNative.ErrEmailRateLimited)

	assertIntakeRequests(t, env, 1)
	assert.Equal(t, storedSubmissionsPerIP, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address),
		"the refusal takes nothing from the budget")
	assert.Zero(t, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeEmail, over.GuardianEmail),
		"the refusal rolls the e-mail count back with it")
}

func TestSubmitRefusedAttemptsTakeNothingFromTheLimits(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	const address = "203.0.113.22"

	// More refused attempts than the e-mail address may store in a day.
	for i := range storedSubmissionsPerEmail + 2 {
		invalid := familySubmission(t, env.phaseID, address, 0)
		invalid.Children = nil
		_, err := env.svc.Submit(testpkg.Ctx(t), invalid)
		require.ErrorIs(t, err, enrollmentNative.ErrInvalidSubmission, "attempt %d", i+1)
	}
	assert.Zero(t, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address))
	assert.Zero(t, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeEmail, "familie0@example.com"))

	_, err := env.svc.Submit(testpkg.Ctx(t), familySubmission(t, env.phaseID, address, 0))
	require.NoError(t, err, "the corrected submission passes")
	assert.Equal(t, 1, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeEmail, "familie0@example.com"))
}

func TestSubmitParentAccountDoesNotCountAgainstTheAddress(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	const address = "203.0.113.23"
	fillSubmissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address, storedSubmissionsPerIP)

	anonymous := familySubmission(t, env.phaseID, address, 1)
	_, err := env.svc.Submit(testpkg.Ctx(t), anonymous)
	require.ErrorIs(t, err, enrollmentNative.ErrRateLimited, "the address is exhausted for a submission without a session")

	signedIn := familySubmission(t, env.phaseID, address, 2)
	signedIn.GuardianAccountID = &env.creatorID
	_, err = env.svc.Submit(testpkg.Ctx(t), signedIn)
	require.NoError(t, err, "a parent with an account passes the exhausted address")
	assert.Equal(t, storedSubmissionsPerIP, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address),
		"and does not count against it")
	assert.Equal(t, 1, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeEmail, signedIn.GuardianEmail),
		"the e-mail address still counts")

	// The e-mail limit holds for a parent with an account too.
	fillSubmissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeEmail, "familie3@example.com", storedSubmissionsPerEmail)
	flooded := familySubmission(t, env.phaseID, address, 3)
	flooded.GuardianAccountID = &env.creatorID
	_, err = env.svc.Submit(testpkg.Ctx(t), flooded)
	require.ErrorIs(t, err, enrollmentNative.ErrEmailRateLimited)
}

func TestSubmitParallelSubmissionsCannotPassTheLimitTogether(t *testing.T) {
	t.Parallel()
	env := setupRequestTest(t)
	const (
		address  = "203.0.113.24"
		left     = 3
		families = 9
	)
	fillSubmissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address, storedSubmissionsPerIP-left)

	requests := make([]enrollmentAPI.SubmitRequest, families)
	for i := range requests {
		requests[i] = familySubmission(t, env.phaseID, address, i)
	}
	errs := make([]error, families)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = env.svc.Submit(testpkg.Ctx(t), requests[i])
		}()
	}
	close(start)
	wg.Wait()

	stored := 0
	for i, err := range errs {
		if err == nil {
			stored++
			continue
		}
		require.ErrorIs(t, err, enrollmentNative.ErrRateLimited, "family %d", i)
	}
	assert.Equal(t, left, stored, "exactly the submissions the budget had left are stored")
	assertIntakeRequests(t, env, left)
	assert.Equal(t, storedSubmissionsPerIP, submissionBucket(t, env.db, enrollmentNative.SubmissionRateLimitKeyTypeIP, address))
}

// assertIntakeRequests requires that the phase of env holds requests stored
// requests, each with its one child.
func assertIntakeRequests(t *testing.T, env *requestTestEnv, requests int) {
	t.Helper()
	var requestCount, childCount int
	require.NoError(t, env.db.NewRaw("SELECT COUNT(*) FROM enrollment.requests WHERE phase_id = ?", env.phaseID).Scan(testpkg.Ctx(t), &requestCount))
	require.NoError(t, env.db.NewRaw(`SELECT COUNT(*) FROM enrollment.request_children c
		JOIN enrollment.requests r ON r.id = c.request_id WHERE r.phase_id = ?`, env.phaseID).Scan(testpkg.Ctx(t), &childCount))
	assert.Equal(t, requests, requestCount, "stored requests")
	assert.Equal(t, requests, childCount, "stored children")
}
