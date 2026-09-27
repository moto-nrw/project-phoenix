package behavior_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"path/filepath"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The welcome mail (#3534) follows a new invitation: exactly one per
// invitation for staff, at most one per guardian and school for parents,
// and never again on a resend.

// welcomeSchoolSettings gives the school settings that differ from the
// registry defaults, so the help link has to carry the school's values.
func welcomeSchoolSettings(t *testing.T, module services.AuthTestModule) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.Settings.SetValue(ctx, "attendance.nfc_enabled", true, nil, nil))
	require.NoError(t, module.Settings.SetValue(ctx, "operations.presence_mode", "binary", nil, nil))
}

const welcomeSettingsQuery = "nfc_enabled=true&presence_mode=binary&group_mode=fixed_groups"

func countTemplate(messages []testpkg.EmailMessage, name string) int {
	count := 0
	for _, message := range messages {
		if message.Template == name {
			count++
		}
	}
	return count
}

func TestCreateSchoolInvitationSendsOneWelcomeAndResendDoesNot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mailer := testpkg.NewCapturingMailer()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db),
		services.WithAuthTestMailer(mailer),
		services.WithAuthTestPasswordResetBackoff(time.Millisecond),
	)
	require.NoError(t, err)
	welcomeSchoolSettings(t, module)
	ctx := testpkg.Ctx(t)
	_, err = db.NewRaw(`UPDATE platform.schools SET name = 'OGS Am Berg' WHERE id = ?`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	creator := testpkg.CreateTestAccount(t, db, "welcome-creator")
	role := testpkg.CreateTestRole(t, db, "welcome-caregiver")
	address := inviteeAddress("welcome")

	invitation, err := module.Invitation.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: address, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	require.True(t, mailer.WaitForMessages(2, 5*time.Second), "create sends the invitation and the welcome")

	welcome, ok := mailer.MessageWithTemplate("staff-welcome.html")
	require.True(t, ok)
	assert.Equal(t, address, welcome.To.Address)
	assert.Equal(t, "Willkommen bei moto – OGS Am Berg", welcome.Subject)
	helpURL, _ := welcome.Content.(map[string]any)["HelpURL"].(string)
	assert.Contains(t, helpURL, "/help/einladung-annehmen-und-konto-einrichten?role=caregiver&"+welcomeSettingsQuery,
		"a role without lead rights opens the caregiver article with the school's settings")
	assert.NotContains(t, helpURL, "return_to")
	assert.Equal(t, 1, countTemplate(mailer.Messages(), "invitation.html"))

	require.NoError(t, module.Invitation.ResendSchoolInvitation(ctx, invitation.ID, creator.ID))
	require.True(t, mailer.WaitForMessages(3, 5*time.Second), "the resend mails the link again")
	// Give a stray welcome time to arrive before counting.
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 2, countTemplate(mailer.Messages(), "invitation.html"))
	assert.Equal(t, 1, countTemplate(mailer.Messages(), "staff-welcome.html"), "a resend never welcomes again")
}

// A role that carries config:manage is a lead, whatever the school calls it.
func TestSchoolWelcomeOpensTheLeadArticleForALeadRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mailer := testpkg.NewCapturingMailer()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db),
		services.WithAuthTestMailer(mailer),
		services.WithAuthTestPasswordResetBackoff(time.Millisecond),
	)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "welcome-lead-creator")
	role := testpkg.CreateTestRole(t, db, "welcome-lead")
	_, err = db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ?, id FROM auth.permissions WHERE name = 'config:manage'`, role.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = module.Invitation.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("welcome-lead"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Grace"), LastName: testpkg.StrPtr("Hopper"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	require.True(t, mailer.WaitForMessages(2, 5*time.Second))
	welcome, ok := mailer.MessageWithTemplate("staff-welcome.html")
	require.True(t, ok)
	helpURL, _ := welcome.Content.(map[string]any)["HelpURL"].(string)
	assert.Contains(t, helpURL, "/help/einladung-annehmen-und-konto-einrichten?role=lead&")
}

type outboxRow struct {
	ID                int64  `bun:"id"`
	Kind              string `bun:"kind"`
	RelatedEntityType string `bun:"related_entity_type"`
	RelatedEntityID   int64  `bun:"related_entity_id"`
}

// guardianOutbox reads the queued mails of one guardian contact: its
// invitations and its welcome.
func guardianOutbox(t *testing.T, db *bun.DB, guardianProfileID int64) []outboxRow {
	t.Helper()
	var rows []outboxRow
	require.NoError(t, db.NewRaw(`
		SELECT o.id, o.kind, COALESCE(o.related_entity_type, '') AS related_entity_type, COALESCE(o.related_entity_id, 0) AS related_entity_id
		FROM platform.email_outbox o
		WHERE (o.related_entity_type = 'guardian_profile' AND o.related_entity_id = ?)
		   OR (o.related_entity_type = 'guardian_invitation' AND o.related_entity_id IN (
				SELECT id FROM auth.guardian_invitations WHERE guardian_profile_id = ?))
		ORDER BY o.id`, guardianProfileID, guardianProfileID).Scan(context.Background(), &rows))
	return rows
}

func countKind(rows []outboxRow, kind string) int {
	count := 0
	for _, row := range rows {
		if row.Kind == kind {
			count++
		}
	}
	return count
}

func TestGuardianInvitationQueuesOneWelcomePerGuardianAndSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	welcomeSchoolSettings(t, module)
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "welcome-guardian")
	creator := testpkg.CreateTestAccount(t, db, "welcome-guardian-creator")

	// The routes run the flows in the request's tenant transaction, which
	// the outbox joins.
	inRequest := func(run func(txCtx context.Context) error) {
		t.Helper()
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			return run(txCtx)
		}))
	}
	var invitation identityaccess.GuardianInvitation
	inRequest(func(txCtx context.Context) error {
		var err error
		invitation, err = module.GuardianInvitation.CreateGuardianInvitation(txCtx, identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creator.ID,
		})
		return err
	})
	rows := guardianOutbox(t, db, profile.ID)
	require.Len(t, rows, 2, "the invitation and the welcome")
	assert.Equal(t, "guardian_invitation", rows[0].Kind)
	assert.Equal(t, "guardian_welcome", rows[1].Kind, "the welcome is queued after the invitation")

	var raw string
	require.NoError(t, db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE id = ?`, rows[1].ID).Scan(ctx, &raw))
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	assert.Contains(t, payload["help_url"], "/help/eltern-konto-einrichten?role=parent&"+welcomeSettingsQuery)
	assert.Contains(t, payload["parent_info_url"], "/downloads/moto-elterninfo.pdf")

	inRequest(func(txCtx context.Context) error {
		return module.GuardianInvitation.ResendGuardianInvitation(txCtx, invitation.ID, creator.ID)
	})
	rows = guardianOutbox(t, db, profile.ID)
	assert.Equal(t, 2, countKind(rows, "guardian_invitation"), "the resend mails the link again")
	assert.Equal(t, 1, countKind(rows, "guardian_welcome"), "a resend never welcomes again")

	// A new invitation for the same guardian replaces the old link; the
	// guardian was already welcomed to this school.
	inRequest(func(txCtx context.Context) error {
		_, err := module.GuardianInvitation.CreateGuardianInvitation(txCtx, identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creator.ID,
		})
		return err
	})
	rows = guardianOutbox(t, db, profile.ID)
	assert.Equal(t, 3, countKind(rows, "guardian_invitation"))
	assert.Equal(t, 1, countKind(rows, "guardian_welcome"), "one welcome per guardian and school")
}

// A welcome that fails to queue leaves the invitation and its mail intact
// (#3534): the failed statement is rolled back to a savepoint instead of
// aborting the invitation's transaction.
func TestGuardianWelcomeFailureKeepsTheInvitation(t *testing.T) {
	t.Parallel()
	// The welcome breaks with a failing statement on the invitation's
	// transaction, the way a database error inside the enqueue would.
	outbox := testpkg.NewCapturingOutbox()
	outbox.FailKind("guardian_welcome", func(ctx context.Context) error {
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return errors.New("welcome enqueue ran outside the invitation transaction")
		}
		_, err := raw.(bun.IDB).ExecContext(ctx, "SELECT 1/0")
		return err
	})
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *services.GuardianInvitationTestConfig) {
		cfg.Outbox = outbox
	})
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "welcome-failure")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)

	var invitation identityaccess.GuardianInvitation
	require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var err error
		invitation, err = env.service.CreateGuardianInvitation(txCtx, identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creatorID,
		})
		return err
	}), "the invitation commits although its welcome failed")
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	stored := testpkg.GuardianInvitationByID(t, env.db, invitation.ID)
	assert.Equal(t, invitation.Token, stored.Token, "the invitation is stored")
	require.Len(t, outbox.Requests(), 1, "the invitation mail is still queued")
	assert.Equal(t, "guardian_invitation", outbox.Requests()[0].Kind)
}

// The queued welcome renders the greeting, the help article and the
// Elterninfo; it carries no accept link.
func TestGuardianWelcomeRendersHelpAndParentInfo(t *testing.T) {
	t.Parallel()
	outbox := testpkg.NewCapturingOutbox()
	mailer := services.NewGuardianInvitationMailer(services.GuardianInvitationMailerConfig{
		Outbox: outbox, FrontendURL: "https://eltern.example.test/",
	})
	require.NoError(t, mailer.EnqueueWelcome(context.Background(), 42, services.GuardianMailRecipient{
		FirstName: " Olga ", LastName: "Muster", Email: " olga@example.test ",
	}, "OGS Musterschule", "https://eltern.example.test/help/eltern-konto-einrichten?role=parent"))

	require.Len(t, outbox.Requests(), 1)
	req := outbox.Requests()[0]
	assert.Equal(t, "guardian_welcome", req.Kind)
	assert.Equal(t, "guardian_profile", req.RelatedEntityType)
	assert.Equal(t, int64(42), req.RelatedEntityID)
	assert.Equal(t, "guardian_welcome:42", req.IdempotencyKey)
	assert.NotContains(t, req.Payload, "invitation_url")
	assert.Equal(t, "https://eltern.example.test/downloads/moto-elterninfo.pdf", req.Payload["parent_info_url"])

	render := services.NewGuardianWelcomeRenderer(services.GuardianInvitationRendererConfig{})
	msg, err := render(context.Background(), req.Payload)
	require.NoError(t, err)
	assert.Equal(t, "Willkommen bei moto – OGS Musterschule", msg.Subject)
	assert.Equal(t, "guardian-welcome.html", msg.Template)
	assert.Equal(t, "olga@example.test", msg.To.Address)

	body := renderWelcomeTemplate(t, msg)
	assert.Contains(t, body, "Guten Tag Olga Muster")
	assert.Contains(t, body, `href="https://eltern.example.test/help/eltern-konto-einrichten?role=parent"`)
	assert.Contains(t, body, `href="https://eltern.example.test/downloads/moto-elterninfo.pdf"`)
	assert.Contains(t, body, "Zur Hilfe")
	assert.NotContains(t, body, "Einladung annehmen")

	_, err = render(context.Background(), map[string]any{"recipient_email": "olga@example.test"})
	require.Error(t, err, "a welcome without help link is not sent")
}

func TestStaffWelcomeTemplateRendersHelpLink(t *testing.T) {
	t.Parallel()
	body := renderWelcomeTemplate(t, &testpkg.EmailMessage{
		Template: "staff-welcome.html",
		Content: map[string]any{
			"HelpURL":           "https://moto.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver",
			"FirstName":         "Ada",
			"SchoolName":        "OGS Am Berg",
			"ReplyGoesToSchool": true,
		},
	})
	assert.Contains(t, body, "Hallo Ada,")
	assert.Contains(t, body, "OGS Am Berg")
	assert.Contains(t, body, `href="https://moto.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver"`)
	assert.Contains(t, body, "Die Antwort geht an OGS Am Berg.", "the reply goes to the school the Reply-To names")

	withoutReplyTo := renderWelcomeTemplate(t, &testpkg.EmailMessage{
		Template: "staff-welcome.html",
		Content: map[string]any{
			"HelpURL": "https://moto.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver", "SchoolName": "OGS Am Berg",
		},
	})
	assert.NotContains(t, withoutReplyTo, "Antworte einfach", "without a school reply address a reply would reach no one at the school")
}

func renderWelcomeTemplate(t *testing.T, msg *testpkg.EmailMessage) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "templates", "email")
	tmpl, err := template.ParseFiles(
		filepath.Join(dir, "styles.html"), filepath.Join(dir, "header.html"),
		filepath.Join(dir, "footer.html"), filepath.Join(dir, msg.Template),
	)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tmpl.ExecuteTemplate(&buf, msg.Template, msg.Content))
	return buf.String()
}
