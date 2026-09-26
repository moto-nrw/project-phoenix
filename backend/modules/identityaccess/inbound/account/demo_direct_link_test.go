package account_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The direct link: a request that prepares a new demo school answers the
// mailed link as well, so the website can send the visitor straight in. A
// request that reuses the address's school, and one within the cooldown,
// answer without it; the mail leaves exactly as before.

// schoolOf is the demo school the token's access enters.
func (e demoEnv) schoolOf(t *testing.T, token string) string {
	t.Helper()
	var slug string
	require.NoError(t, e.db.NewRaw(`SELECT school_slug FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &slug))
	return slug
}

// syncBuffer is a log sink the mail goroutines may write to while the test
// reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestDemoAccessRequestForANewSchoolAnswersTheMailedLink(t *testing.T) {
	t.Parallel()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := mountDemoEnv(t, testpkg.SetupTestDB(t), "", testutil.WithAuthTestLogger(logger))
	body := env.requestBodyFor(t, env.address())
	body["role"] = "lead"

	rr := env.post(t, "/demo/access-requests", body)
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), "link and lead mail: %v", env.mails.Templates())
	links := env.mailedLinks()
	require.Len(t, links, 1)
	assert.JSONEq(t, `{"link_sent":true,"link":"`+links[0]+`"}`, rr.Body.String(), "the answer carries the mailed link")

	token, role, found := strings.Cut(strings.TrimPrefix(links[0], demoEntryPrefix), "&role=")
	require.True(t, found, links[0])
	assert.Equal(t, "lead", role, "the answered link keeps the preselected role, as the mailed one does")
	assert.JSONEq(t, `{"status":"preparing","school_name":"OGS Beispiel"}`, env.status(token).Body.String(),
		"the answered link enters the school this request queued")
	assert.NotContains(t, logs.String(), token, "the token leaves in the answer and the mail, never in a log")
}

// An address with a school gets its further link by mail only, whether the
// school is still being prepared or ready: whoever typed the address must not
// enter the school of the person it belongs to.
func TestDemoAccessRequestIntoTheAddressesSchoolAnswersNoLink(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	first, answered := env.requestAccess(t, env.requestBodyFor(t, env.address()))
	require.NotEmpty(t, answered, "the first request queues a school")
	slug := env.schoolOf(t, first)

	env.endCooldown(t)
	rr := env.post(t, "/demo/access-requests", env.requestBodyFor(t, env.address()))
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"link_sent":true}`, rr.Body.String(), "a preparing school's link leaves by mail only")
	require.Eventually(t, func() bool { return len(env.mailedLinks()) == 2 }, 2*time.Second, 10*time.Millisecond,
		"the link is still mailed: %v", env.mails.Templates())
	second := strings.TrimPrefix(env.mailedLinks()[1], demoEntryPrefix)
	assert.NotEqual(t, first, second)
	assert.Equal(t, slug, env.schoolOf(t, second))
	assert.NotContains(t, rr.Body.String(), second, "the mailed token stays out of the answer")

	_, visitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, testpkg.Tenant(t))
	seedDemoSchool(t, env.db, slug, testpkg.Tenant(t), visitor.ID)
	require.Contains(t, env.status(first).Body.String(), `"status":"ready"`)
	env.endCooldown(t)
	third, answered := env.requestAccess(t, env.requestBodyFor(t, env.address()))
	assert.Empty(t, answered, "a ready school's link leaves by mail only")
	assert.Equal(t, slug, env.schoolOf(t, third))
}

// Within the cooldown nothing is stored or mailed, so there is no link to
// answer either.
func TestDemoAccessRequestWithinTheCooldownAnswersNoLink(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, answered := env.requestAccess(t, env.requestBodyFor(t, env.address()))
	require.NotEmpty(t, answered)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), env.mails.Templates())
	env.mails.Clear()

	rr := env.post(t, "/demo/access-requests", env.requestBodyFor(t, env.address()))
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"link_sent":true}`, rr.Body.String())
	assert.False(t, env.mails.WaitForMessages(1, 300*time.Millisecond), "no mail within the cooldown: %v", env.mails.Templates())
	assert.Equal(t, 1, env.accessCount(t))
}

// A school that failed for good is not reused: the next request queues a new
// one and answers its link.
func TestDemoAccessRequestAfterAFailedSchoolAnswersTheNewLink(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	first, _ := env.requestAccess(t, env.requestBodyFor(t, env.address()))
	failed := env.schoolOf(t, first)
	_, err := env.db.NewRaw(`UPDATE platform.demo_school_states SET status = 'failed' WHERE name = ?`, failed).Exec(context.Background())
	require.NoError(t, err)
	env.endCooldown(t)

	second, answered := env.requestAccess(t, env.requestBodyFor(t, env.address()))
	require.NotEmpty(t, answered, "a new school answers its link")
	assert.Equal(t, demoEntryPrefix+second, answered)
	assert.NotEqual(t, failed, env.schoolOf(t, second))
}
