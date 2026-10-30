package messaging_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	// E-Mail an den Sorgeberechtigten, wenn die OGS eine Nachricht schreibt
	// (#2307). Die E-Mail ist der Rueckfall, wenn Push nicht eingerichtet ist:
	// ohne sie bleibt eine Nachricht unbemerkt, genau der von einer Schule
	// gemeldete Fall. Getestet wird gegen die echte Datenbank und den echten
	// Dispatcher, nur der Mailer ist der geteilte test.CapturingMailer.

	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"

	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const testParentsURL = "https://eltern.example"

// stubMessagePreferences answers only FilterNotOptedOut, the consent question an
// e-mail channel asks: it drops the accounts that explicitly declined and keeps
// everyone who never decided.
type stubMessagePreferences struct {
	communication.NotificationPreferences
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
	db     *bun.DB
	svc    *messaging.Service
	outbox *api.CapturingOutbox
	chain  testpkg.ParentChain
	staff  int64
}

func newEmailFixture(t *testing.T, preferences communication.NotificationPreferences) *emailFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposAccounts := api.NewAccountSuiteRepositories(testpkg.NewDatabase(t, db))
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	outbox := api.NewCapturingOutbox()
	svc := messaging.NewService(messaging.Config{
		Leaders:     communicationtest.Leaders{},
		UserContext: owners.StaffStudentReads{},
		Session:     api.CommunicationSession{},
		ThreadRepo:  messaging.NewThreadRepository(reposMessages.ParentMessageThread),
		MessageRepo: messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
		ReadRepo:    messaging.NewReadRepository(reposMessages.ParentMessageRead),
		Persons:     newPersons(reposPeople, reposAccounts, db),
		Settings:    settingsValues.NewParentMessagingValues(stubSettings{messagingEnabled: true}),
		Broadcaster: testpkg.NewRecordingBroadcaster(),

		Logger:           slog.Default(),
		Preferences:      preferences,
		Outbox:           api.BindCommunicationCapturingOutbox(outbox),
		GuardianProfiles: owners.BindParentMessageGuardianProfiles(reposPeople.GuardianProfile),
		Schools:          schoolNames{schools: organizations},
		ParentsURL:       testParentsURL,
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	return &emailFixture{db: db, svc: svc, outbox: outbox, chain: chain, staff: staffAccount.ID}
}

// renderedMail is what the delivery worker would send for a captured request.
type renderedMail struct {
	To       string
	Subject  string
	Template string
	Content  map[string]any
}

// renderMessageRow renders a captured outbox request the way the delivery
// worker does. The sender address is the worker's default and stays unset.
func renderMessageRow(t *testing.T, kind string, payload map[string]any) renderedMail {
	t.Helper()
	renderer := messaging.NewParentMessageRenderer(messaging.ParentMessageRendererConfig{}, deliveryPolicy.ParentDeliveryPolicy{})
	msg, err := api.RenderCommunicationMailFixture(context.Background(), communication.MailRenderer(renderer), kind, payload)
	require.NoError(t, err)
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok, "Template-Daten sind eine Map")
	return renderedMail{To: msg.To.Address, Subject: msg.Subject, Template: msg.Template, Content: content}
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

	_, err := testpkg.TenantQuery4(t, adminCtx(t, f.staff), f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Guten Tag, Felix hat heute seine Jacke vergessen. Bitte melden Sie sich kurz bei uns.", "")
	require.NoError(t, err)

	rows := f.outbox.Requests()
	require.Len(t, rows, 1)
	assert.Equal(t, "parent_message", rows[0].Kind)
	assert.Equal(t, "parent_message", rows[0].RelatedEntityType)

	msg := renderMessageRow(t, rows[0].Kind, rows[0].Payload)
	assert.Equal(t, guardianEmail(t, db, f.chain.AccountID), msg.To)
	assert.Equal(t, "Neue Nachricht von der OGS", msg.Subject)
	assert.Equal(t, "parent-message-notification.html", msg.Template)
	content := msg.Content
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

	_, err = testpkg.TenantQuery4(t, adminCtx(t, f.staff), f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Please reply", "")
	require.NoError(t, err)
	rows := f.outbox.Requests()
	require.Len(t, rows, 1)
	msg := renderMessageRow(t, rows[0].Kind, rows[0].Payload)
	assert.Equal(t, "New message from the OGS", msg.Subject)
	content := msg.Content
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

	detail, err := testpkg.TenantQuery4(t, ctx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Erste Nachricht", "")
	require.NoError(t, err)
	before := len(f.outbox.Requests())

	sensitive := "Gesundheitsinformation, die nicht in eine E-Mail gehoert"
	_, err = testpkg.TenantQuery3(t, ctx, f.db, f.svc.PostMessage, detail.ThreadID, sensitive, detail.Messages[0].ID)
	require.NoError(t, err)

	rows := f.outbox.Requests()[before:]
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

	_, err := testpkg.TenantQuery4(t, adminCtx(t, f.staff), f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Guten Tag", "")
	require.NoError(t, err)

	assert.Empty(t, f.outbox.Requests(), "kein Versand nach Widerspruch")
	assert.Equal(t, communication.TypeParentMessage, preferences.gotType)
}

// schoolNames reads the sending school's name through the Organisation &
// Tenancy capability, as the serving root does.
type schoolNames struct{ schools organizationtenancy.Query }

func (n schoolNames) FindSchoolName(ctx context.Context, id int64) (string, bool, error) {
	school, err := n.schools.FindSchool(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return school.Name, true, nil
}
