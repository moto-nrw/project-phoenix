package auth_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The reset mail is built and sent by the composition root over the Delivery
// dispatcher (#2722); the link must land on the portal the account signs in
// to, and a failed send must be readable on the link afterwards.

// flakyMailer fails the first failures sends.
type flakyMailer struct {
	mu       sync.Mutex
	failures int
	attempts int
	err      error
}

func (m *flakyMailer) Send(email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if m.attempts <= m.failures {
		return m.err
	}
	return nil
}

func (m *flakyMailer) sent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

func resetMailContent(t *testing.T, message email.Message) map[string]any {
	t.Helper()
	content, ok := message.Content.(map[string]any)
	require.True(t, ok, "the reset mail carries its template content")
	return content
}

func storedResetDelivery(t *testing.T, db *bun.DB, id int64) (sentAt *time.Time, failure *string, retries int) {
	t.Helper()
	var row struct {
		EmailSentAt     *time.Time `bun:"email_sent_at"`
		EmailError      *string    `bun:"email_error"`
		EmailRetryCount int        `bun:"email_retry_count"`
	}
	require.NoError(t, db.NewRaw(`SELECT email_sent_at, email_error, email_retry_count FROM auth.password_reset_tokens WHERE id = ?`, id).
		Scan(context.Background(), &row))
	return row.EmailSentAt, row.EmailError, row.EmailRetryCount
}

// Every portal keeps its own host: a parent's link must not point at the
// staff frontend, where the account cannot sign in.
func TestPasswordResetMailLinksToTheScopesPortal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mailer := testpkg.NewCapturingMailer()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db), services.WithAuthTestMailer(mailer))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	staff := resetAccount(t, db, "reset-mail-staff")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	forgetResetWindow(t, db, parent.Email)

	link, err := module.AccountAuthentication.InitiatePasswordReset(ctx, staff, identityaccess.PasswordResetScopeStaff)
	require.NoError(t, err)
	require.NotNil(t, link)
	require.True(t, mailer.WaitForMessages(1, 2*time.Second))
	message := mailer.Messages()[0]
	assert.Equal(t, "Passwort zurücksetzen", message.Subject)
	assert.Equal(t, "password-reset.html", message.Template)
	assert.Equal(t, staff, message.To.Address)
	content := resetMailContent(t, message)
	resetURL, _ := content["ResetURL"].(string)
	assert.Contains(t, resetURL, "/reset-password?token="+link.Token)
	assert.Positive(t, content["ExpiryMinutes"], "the mail names the link's lifetime")

	parentLink, err := module.AccountAuthentication.InitiatePasswordReset(ctx, parent.Email, identityaccess.PasswordResetScopeParent)
	require.NoError(t, err)
	require.NotNil(t, parentLink)
	require.True(t, mailer.WaitForMessages(2, 2*time.Second))
	parentURL, _ := resetMailContent(t, mailer.Messages()[1])["ResetURL"].(string)
	assert.Contains(t, parentURL, "/reset-password?token="+parentLink.Token)
	assert.NotEqual(t, hostOf(resetURL), hostOf(parentURL), "the parents portal has its own host")
}

func hostOf(rawURL string) string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(rawURL, "http://"), "https://")
	if index := strings.Index(trimmed, "/"); index >= 0 {
		return trimmed[:index]
	}
	return trimmed
}

// A send that keeps failing records the failure and the retry count on the
// link, so an operator sees why the mail never arrived.
func TestPasswordResetMailFailureIsRecordedOnTheLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mailer := &flakyMailer{failures: 3, err: errors.New("smtp down")}
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db),
		services.WithAuthTestMailer(mailer),
		services.WithAuthTestPasswordResetBackoff(10*time.Millisecond, 20*time.Millisecond, 40*time.Millisecond))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	email := resetAccount(t, db, "reset-mail-failure")

	link, err := module.AccountAuthentication.InitiatePasswordReset(ctx, email, identityaccess.PasswordResetScopeStaff)
	require.NoError(t, err)
	require.NotNil(t, link)

	require.Eventually(t, func() bool { return mailer.sent() == 3 }, 2*time.Second, 10*time.Millisecond,
		"the send is retried three times")
	require.Eventually(t, func() bool {
		_, failure, retries := storedResetDelivery(t, db, link.ID)
		return failure != nil && retries == 3
	}, 2*time.Second, 20*time.Millisecond)
	sentAt, failure, retries := storedResetDelivery(t, db, link.ID)
	assert.Nil(t, sentAt)
	require.NotNil(t, failure)
	assert.Contains(t, *failure, "smtp down")
	assert.Equal(t, 3, retries)
}

// A successful send stamps the link instead.
func TestPasswordResetMailSuccessIsRecordedOnTheLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mailer := testpkg.NewCapturingMailer()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db), services.WithAuthTestMailer(mailer))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	email := resetAccount(t, db, "reset-mail-success")

	link, err := module.AccountAuthentication.InitiatePasswordReset(ctx, email, identityaccess.PasswordResetScopeStaff)
	require.NoError(t, err)
	require.NotNil(t, link)

	require.Eventually(t, func() bool {
		sentAt, _, _ := storedResetDelivery(t, db, link.ID)
		return sentAt != nil
	}, 2*time.Second, 20*time.Millisecond)
	sentAt, failure, retries := storedResetDelivery(t, db, link.ID)
	require.NotNil(t, sentAt)
	assert.Nil(t, failure)
	assert.Equal(t, 1, retries)
}
