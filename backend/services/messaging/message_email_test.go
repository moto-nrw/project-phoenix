package messaging_test

// E-Mail an den Sorgeberechtigten, wenn die OGS eine Nachricht schreibt
// (#2307). Die E-Mail ist der Rueckfall, wenn Push nicht eingerichtet ist:
// ohne sie bleibt eine Nachricht unbemerkt, genau der von einer Schule
// gemeldete Fall. Getestet wird gegen die echte Datenbank und den echten
// Dispatcher, nur der Mailer ist der geteilte test.CapturingMailer.

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/services/messaging"
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
	outbox *recordingMessageOutbox
	chain  testpkg.ParentChain
	staff  int64
}

type recordingMessageOutbox struct {
	mu   sync.Mutex
	rows []platformModels.OutboxEnqueueRequest
}

func (o *recordingMessageOutbox) EnqueueOutbox(_ context.Context, req platformModels.OutboxEnqueueRequest) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rows = append(o.rows, req)
	return nil
}

func (o *recordingMessageOutbox) Rows() []platformModels.OutboxEnqueueRequest {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]platformModels.OutboxEnqueueRequest(nil), o.rows...)
}

func (o *recordingMessageOutbox) Clear() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rows = nil
}

func newEmailFixture(t *testing.T, preferences notifications.PreferenceService) *emailFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	outbox := &recordingMessageOutbox{}
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
		Outbox:           outbox,
		GuardianProfiles: repos.GuardianProfile,
		Schools:          repos.School,
		ParentsURL:       testParentsURL,
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	return &emailFixture{svc: svc, outbox: outbox, chain: chain, staff: staffAccount.ID}
}

// mailContent unwraps the template data of a captured message.
func mailContent(t *testing.T, msg *email.Message) map[string]any {
	t.Helper()
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok, "Template-Daten sind eine Map")
	return content
}

func renderMessageRow(t *testing.T, req platformModels.OutboxEnqueueRequest) *email.Message {
	t.Helper()
	row := &platformModels.EmailOutbox{Kind: req.Kind, Payload: req.Payload}
	renderer := messaging.NewParentMessageRenderer(messaging.ParentMessageRendererConfig{
		DefaultFrom: email.NewEmail("moto", "no-reply@moto.test"),
	})
	msg, err := renderer(context.Background(), row)
	require.NoError(t, err)
	return msg
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
	t.Parallel()

	f := newEmailFixture(t, nil)
	db := testpkg.SetupTestDB(t)

	_, err := f.svc.StartThread(adminCtx(t, f.staff), f.chain.StudentID, f.chain.AccountID,
		"Guten Tag, Felix hat heute seine Jacke vergessen. Bitte melden Sie sich kurz bei uns.")
	require.NoError(t, err)

	rows := f.outbox.Rows()
	require.Len(t, rows, 1)
	assert.Equal(t, platformModels.EmailKindParentMessage, rows[0].Kind)
	assert.Equal(t, platformModels.EmailRelatedTypeParentMessage, rows[0].RelatedEntityType)

	msg := renderMessageRow(t, rows[0])
	assert.Equal(t, guardianEmail(t, db, f.chain.AccountID), msg.To.Address)
	assert.Equal(t, "Neue Nachricht von der OGS", msg.Subject)
	assert.Equal(t, "parent-message-notification.html", msg.Template)
	content := mailContent(t, msg)
	assert.Equal(t, testParentsURL+"/messages/"+strconv.FormatInt(f.chain.StudentID, 10), content["MessagesURL"])
	assert.NotContains(t, content, "ChildName")
	assert.NotContains(t, content, "Preview")
	assert.NotContains(t, rows[0].Payload, "body")
}

func TestStartThread_LocalizesGuardianEmail(t *testing.T) {
	t.Parallel()

	f := newEmailFixture(t, nil)
	db := testpkg.SetupTestDB(t)
	_, err := db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", "en").
		Where("account_id = ?", f.chain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = f.svc.StartThread(adminCtx(t, f.staff), f.chain.StudentID, f.chain.AccountID, "Please reply")
	require.NoError(t, err)
	rows := f.outbox.Rows()
	require.Len(t, rows, 1)
	msg := renderMessageRow(t, rows[0])
	assert.Equal(t, "New message from the OGS", msg.Subject)
	content := mailContent(t, msg)
	assert.Equal(t, "New message", content["BrandKicker"])
	assert.Equal(t, "Reply", content["ReplyLabel"])
	assert.Contains(t, content["IntroText"], "sent you a message")
	assert.Equal(t, "Powered by", content["PoweredByLabel"])
	assert.Contains(t, content["FooterText"], "This email was sent")
	assert.NotContains(t, content["FooterText"], "Diese E-Mail")
}

func TestPostMessage_QueuesAnotherDataMinimalEmail(t *testing.T) {
	t.Parallel()

	f := newEmailFixture(t, nil)
	ctx := adminCtx(t, f.staff)

	detail, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Erste Nachricht")
	require.NoError(t, err)
	f.outbox.Clear()

	sensitive := "Gesundheitsinformation, die nicht in eine E-Mail gehoert"
	_, err = f.svc.PostMessage(ctx, detail.ThreadID, sensitive, detail.Messages[0].ID)
	require.NoError(t, err)

	rows := f.outbox.Rows()
	require.Len(t, rows, 1)
	assert.NotContains(t, fmt.Sprint(rows[0].Payload), sensitive)
	assert.Contains(t, rows[0].IdempotencyKey, "parent_message:")
}

// TestStartThread_RespectsGuardianOptOut: who switched the notification off is
// not written to by e-mail either.
func TestStartThread_RespectsGuardianOptOut(t *testing.T) {
	t.Parallel()

	preferences := &stubMessagePreferences{declined: true}
	f := newEmailFixture(t, preferences)

	_, err := f.svc.StartThread(adminCtx(t, f.staff), f.chain.StudentID, f.chain.AccountID, "Guten Tag")
	require.NoError(t, err)

	assert.Empty(t, f.outbox.Rows(), "kein Versand nach Widerspruch")
	assert.Equal(t, notifications.TypeParentMessage, preferences.gotType)
}
