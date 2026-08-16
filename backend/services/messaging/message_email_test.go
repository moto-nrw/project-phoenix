package messaging_test

// E-Mail an den Sorgeberechtigten, wenn die OGS eine Nachricht schreibt
// (#2307). Die E-Mail ist der Rueckfall, wenn Push nicht eingerichtet ist:
// ohne sie bleibt eine Nachricht unbemerkt, genau der von einer Schule
// gemeldete Fall. Getestet wird gegen die echte Datenbank und den echten
// Dispatcher, nur der Mailer ist der geteilte test.CapturingMailer.

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/services/messaging"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const testParentsURL = "https://eltern.example"

// stubMessagePreferences answers only FilterNotOptedOut, the consent question an
// e-mail channel asks: it drops the accounts that explicitly declined and keeps
// everyone who never decided.
type stubMessagePreferences struct {
	notifications.PreferenceService
	declined bool
	gotType  string
}

func (s *stubMessagePreferences) FilterNotOptedOut(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	s.gotType = notificationType
	if s.declined {
		return nil, nil
	}
	return accountIDs, nil
}

type emailFixture struct {
	svc    *messaging.Service
	mailer *testpkg.CapturingMailer
	chain  testpkg.ParentChain
	staff  int64
}

func newEmailFixture(t *testing.T, preferences notifications.PreferenceService) *emailFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	mailer := testpkg.NewCapturingMailer()
	svc := messaging.NewService(messaging.Config{
		ThreadRepo:       repos.ParentMessageThread,
		MessageRepo:      repos.ParentMessage,
		ReadRepo:         repos.ParentMessageRead,
		Persons:          newPersons(repos, db),
		Settings:         stubSettings{messagingEnabled: true},
		Broadcaster:      testpkg.NewRecordingBroadcaster(),
		DB:               db,
		Logger:           slog.Default(),
		Preferences:      preferences,
		Dispatcher:       email.NewDispatcher(mailer, slog.Default()),
		GuardianProfiles: repos.GuardianProfile,
		Schools:          repos.School,
		DefaultFrom:      email.NewEmail("moto", "no-reply@moto.test"),
		ParentsURL:       testParentsURL,
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	t.Cleanup(func() { testpkg.CleanupStaffFixtures(t, db, staff.ID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, staffAccount.ID) })
	t.Cleanup(func() { testpkg.CleanupParentMessagingForAccount(t, db, staffAccount.ID) })

	return &emailFixture{svc: svc, mailer: mailer, chain: chain, staff: staffAccount.ID}
}

// mailContent unwraps the template data of a captured message.
func mailContent(t *testing.T, msg email.Message) map[string]any {
	t.Helper()
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok, "Template-Daten sind eine Map")
	return content
}

func guardianEmail(t *testing.T, db *bun.DB, accountID int64) string {
	t.Helper()
	var address string
	err := db.NewSelect().
		ColumnExpr("email").
		TableExpr("auth.accounts").
		Where("id = ?", accountID).
		Scan(context.Background(), &address)
	require.NoError(t, err)
	return address
}

// TestStartThread_SendsGuardianEmail is the reported gap: the OGS writes, and the
// guardian learns about it even without push.
func TestStartThread_SendsGuardianEmail(t *testing.T) {
	f := newEmailFixture(t, nil)
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	_, err := f.svc.StartThread(adminCtx(f.staff), f.chain.StudentID, f.chain.AccountID,
		"Guten Tag, Felix hat heute seine Jacke vergessen. Bitte melden Sie sich kurz bei uns.")
	require.NoError(t, err)

	require.True(t, f.mailer.WaitForMessages(1, 2*time.Second), "genau eine E-Mail wird ausgeloest")
	messages := f.mailer.Messages()
	require.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, guardianEmail(t, db, f.chain.AccountID), msg.To.Address)
	assert.Equal(t, "Neue Nachricht von der OGS", msg.Subject)
	assert.Equal(t, "parent-message-notification.html", msg.Template)
	content := mailContent(t, msg)
	assert.Equal(t, "Felix Schneider", content["ChildName"])
	assert.Equal(t, testParentsURL+"/messages", content["MessagesURL"])
	preview, _ := content["Preview"].(string)
	assert.Contains(t, preview, "Guten Tag")
}

func TestStartThread_LocalizesGuardianEmail(t *testing.T) {
	f := newEmailFixture(t, nil)
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	_, err := db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", "en").
		Where("account_id = ?", f.chain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = f.svc.StartThread(adminCtx(f.staff), f.chain.StudentID, f.chain.AccountID, "Please reply")
	require.NoError(t, err)
	require.True(t, f.mailer.WaitForMessages(1, 2*time.Second))

	msg := f.mailer.Messages()[0]
	assert.Equal(t, "New message from the OGS", msg.Subject)
	content := mailContent(t, msg)
	assert.Equal(t, "New message", content["BrandKicker"])
	assert.Equal(t, "Reply", content["ReplyLabel"])
	assert.Contains(t, content["IntroText"], "sent you a message")
	assert.Equal(t, "Powered by", content["PoweredByLabel"])
	assert.Contains(t, content["FooterText"], "This email was sent")
	assert.NotContains(t, content["FooterText"], "Diese E-Mail")
}

// TestPostMessage_ShortensThePreview keeps the mail a pointer into the portal:
// the full conversation stays behind the login, the mail carries a taste of it.
func TestPostMessage_ShortensThePreview(t *testing.T) {
	f := newEmailFixture(t, nil)
	ctx := adminCtx(f.staff)

	detail, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Erste Nachricht")
	require.NoError(t, err)
	f.mailer.Clear()

	long := strings.Repeat("Sehr ausfuehrliche Elterninformation. ", 20)
	_, err = f.svc.PostMessage(ctx, detail.ThreadID, long, detail.Messages[0].ID)
	require.NoError(t, err)

	require.True(t, f.mailer.WaitForMessages(1, 2*time.Second))
	preview, _ := mailContent(t, f.mailer.Messages()[0])["Preview"].(string)
	assert.Less(t, len([]rune(preview)), len([]rune(long)))
	assert.True(t, strings.HasSuffix(preview, "…"), "gekuerzte Vorschau endet mit Auslassungszeichen")
}

// TestStartThread_RespectsGuardianOptOut: who switched the notification off is
// not written to by e-mail either.
func TestStartThread_RespectsGuardianOptOut(t *testing.T) {
	preferences := &stubMessagePreferences{declined: true}
	f := newEmailFixture(t, preferences)

	_, err := f.svc.StartThread(adminCtx(f.staff), f.chain.StudentID, f.chain.AccountID, "Guten Tag")
	require.NoError(t, err)

	assert.False(t, f.mailer.WaitForMessages(1, 300*time.Millisecond), "kein Versand nach Widerspruch")
	assert.Equal(t, notifications.TypeParentMessage, preferences.gotType)
}
