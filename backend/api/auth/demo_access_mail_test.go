package auth_test

import (
	"context"
	"encoding/json"
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
	assert.Equal(t, demoEntryOrigin+"/demo#token="+token, content["EntryURL"])
	assert.Equal(t, "Kim Beispiel", content["PersonName"])

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

// A known address gets its link by mail only: whoever typed it may not be
// the person it belongs to. The team hears about a demo access once.
func TestDemoAccessRequestOfAKnownAddressMailsTheLinkAgain(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.provisionDemoAdmin(t)
	first := env.requestToken(t)
	require.True(t, env.mails.WaitForMessages(2, 2*time.Second), env.mails.Templates())
	env.mails.Clear()

	rr := env.post(t, "/demo/access-requests", env.requestBody(t))
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Equal(t, map[string]any{"link_sent": true}, response, "no entry_url for a known address")

	require.True(t, env.mails.WaitForMessages(1, 2*time.Second))
	link, ok := env.mails.MessageWithTemplate(demoAccessTemplate)
	require.True(t, ok, env.mails.Templates())
	assert.Equal(t, env.address(), link.To.Address)
	content, ok := link.Content.(map[string]any)
	require.True(t, ok)
	entryURL, _ := content["EntryURL"].(string)
	prefix := demoEntryOrigin + "/demo#token="
	require.Greater(t, len(entryURL), len(prefix))
	mailed := entryURL[len(prefix):]

	rr = env.post(t, "/demo/access/sessions", map[string]string{"token": mailed})
	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	rr = env.post(t, "/demo/access/sessions", map[string]string{"token": first})
	assert.Equal(t, http.StatusNotFound, rr.Code, "only the fingerprint is stored, so the mailed link replaces the first one")

	var accesses int
	require.NoError(t, env.db.NewRaw(`SELECT COUNT(*) FROM auth.demo_accesses WHERE email = ?`, env.address()).Scan(context.Background(), &accesses))
	assert.Equal(t, 1, accesses, "the same demo access, not a second one")
	// The lead mail would follow the link mail at once; give it the same time.
	assert.False(t, env.mails.WaitForMessages(2, 300*time.Millisecond), "no second lead mail: %v", env.mails.Templates())
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
