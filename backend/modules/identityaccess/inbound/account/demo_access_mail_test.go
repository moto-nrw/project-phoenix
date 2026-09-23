package account_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The mails of the public demo (#3465), driven through the wired router with
// the demo environment's mail lock on the transport.

const (
	demoAccessTemplate = "demo-access.html"
	demoLeadTemplate   = "demo-lead.html"
)

func TestDemoAccessRequestMailsTheLinkAndTheLead(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	token := env.requestToken(t)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), env.mails.Templates())

	var accessID int64
	require.NoError(t, env.db.NewRaw(`SELECT id FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &accessID))

	link, ok := env.mails.MessageWithTemplate(demoAccessTemplate)
	require.True(t, ok, env.mails.Templates())
	assert.Equal(t, env.address(), link.To.Address)
	assert.Equal(t, "kontakt@moto.nrw", link.ReplyTo.Address)
	assert.NotEmpty(t, link.From.Address, "the sender of the product mails")
	content, ok := link.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, demoEntryPrefix+token, content["EntryURL"])
	// Anyone can type a foreign address, so nothing the form carried may
	// reach that inbox from our sender.
	assert.Empty(t, link.To.Name)
	assert.NotContains(t, content, "PersonName")

	lead, ok := env.mails.MessageWithTemplate(demoLeadTemplate)
	require.True(t, ok, env.mails.Templates())
	assert.Equal(t, "kontakt@moto.nrw", lead.To.Address)
	assert.Equal(t, link.From, lead.From)
	content, ok = lead.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Kim Beispiel", content["PersonName"])
	assert.Equal(t, "OGS Beispiel", content["OGSName"])
	assert.Equal(t, env.address(), content["Email"])
	assert.Equal(t, "messe", content["Source"])
	assert.Equal(t, true, content["ContactOptIn"])
	assert.Equal(t, accessID, content["AccessID"])
}

func (e demoEnv) accessCount(t *testing.T) int {
	t.Helper()
	var accesses int
	require.NoError(t, e.db.NewRaw(`SELECT COUNT(*) FROM auth.demo_accesses WHERE email = ?`, e.address()).Scan(context.Background(), &accesses))
	return accesses
}

// An address waits between two links: a script that posts it in a loop
// neither floods the inbox nor stores anything, and the answer stays the same.
func TestDemoAccessRequestWithinTheCooldownMailsNothing(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.requestToken(t)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), env.mails.Templates())
	env.mails.Clear()

	rr := env.post(t, "/demo/access-requests", env.requestBody(t))
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"link_sent":true}`, rr.Body.String(), "the answer tells nothing about the address")
	assert.False(t, env.mails.WaitForMessages(1, 300*time.Millisecond), "no mail within the cooldown: %v", env.mails.Templates())
	assert.Equal(t, 1, env.accessCount(t))
}

// After the cooldown a known address gets a further link, and the earlier
// one keeps working: a failed delivery or a stranger typing the address
// locks nobody out. The team hears again only when the consent changed.
func TestDemoAccessRequestOfAKnownAddressKeepsTheEarlierLink(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.provisionDemoAdmin(t)
	first := env.requestToken(t)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), env.mails.Templates())
	env.endCooldown(t)
	env.mails.Clear()

	second := env.requestToken(t)
	assert.NotEqual(t, first, second)
	for _, token := range []string{first, second} {
		rr := env.post(t, "/demo/access/sessions", map[string]string{"token": token})
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	}
	assert.Equal(t, 2, env.accessCount(t), "every request stores its own access")
	// The lead mail would follow the link mail at once; give it the same time.
	assert.False(t, env.mails.WaitForMessages(2, 300*time.Millisecond), "no second lead mail: %v", env.mails.Templates())

	env.endCooldown(t)
	env.mails.Clear()
	withdrawn := env.requestBody(t)
	withdrawn["contact_opt_in"] = false
	third := env.requestTokenWith(t, withdrawn)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), env.mails.Templates())
	lead, ok := env.mails.MessageWithTemplate(demoLeadTemplate)
	require.True(t, ok, "the team hears of a changed consent: %v", env.mails.Templates())
	content, ok := lead.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, content["ContactOptIn"])
	var optIn bool
	require.NoError(t, env.db.NewRaw(`SELECT contact_opt_in FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(third)).Scan(context.Background(), &optIn))
	assert.False(t, optIn, "the withdrawn consent is stored")
}

// The mail lock belongs to the environment: a password reset asked for over
// the public route is processed, but its mail never leaves the server.
func TestDemoEnvironmentKeepsThePasswordResetMailOnTheServer(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	accountID := env.provisionDemoAdmin(t)
	var address string
	require.NoError(t, env.db.NewRaw(`SELECT email FROM auth.accounts WHERE id = ?`, accountID).Scan(context.Background(), &address))
	testpkg.OwnTestPasswordResetTokensForEmail(t, env.db, address)

	rr := testutil.ExecuteRequest(env.router, testutil.NewJSONRequest(t, http.MethodPost, "/auth/password-reset", map[string]string{"email": address}))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Eventually(t, func() bool {
		var settled int
		err := env.db.NewRaw(`SELECT COUNT(*) FROM auth.password_reset_tokens WHERE account_id = ? AND email_sent_at IS NOT NULL`, accountID).Scan(context.Background(), &settled)
		return err == nil && settled == 1
	}, 3*time.Second, 20*time.Millisecond, "the reset mail reached the transport's lock")
	assert.Empty(t, env.mails.Messages(), "the reset mail must not be delivered")

	env.requestToken(t)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second))
	assert.ElementsMatch(t, []string{demoAccessTemplate, demoLeadTemplate}, env.mails.Templates())
}
