package messaging_test

// Integration tests for the personal count scope (#3673): each staff member
// chooses which parent conversations their own counter at "Nachrichten"
// counts. The counts run on the real projection; only the preference store
// and the caller's groups are stand-ins, because the store has its own test in
// the composition package and the groups come from the caller context.

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// memoryCountPreferences keeps count scopes per account, like the real store
// does per (school, account) inside one school.
type memoryCountPreferences struct {
	mu     sync.Mutex
	scopes map[int64]string
}

func (m *memoryCountPreferences) CountScope(_ context.Context, accountID int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.scopes[accountID], nil
}

func (m *memoryCountPreferences) SetCountScope(_ context.Context, accountID int64, scope string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scopes[accountID] = scope
	return nil
}

// groupsByAccount answers MyGroupIDs for the account in the request claims.
type groupsByAccount map[int64][]int64

func (g groupsByAccount) MyGroupIDs(ctx context.Context) ([]int64, error) {
	return g[int64(jwt.ClaimsFromCtx(ctx).ID)], nil
}

// newCountScopeFixture is newFixture with a count preference store and the
// given caller groups wired in. groups is read at count time, so a test may
// fill it after creating its groups.
func newCountScopeFixture(t *testing.T, groups groupsByAccount) *fixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	bc := testpkg.NewRecordingBroadcaster()
	svc := messaging.NewService(messaging.Config{
		ThreadRepo:       repos.ParentMessageThread,
		MessageRepo:      repos.ParentMessage,
		ReadRepo:         repos.ParentMessageRead,
		Persons:          newPersons(repos, db),
		Settings:         stubSettings{messagingEnabled: true},
		Broadcaster:      bc,
		DB:               db,
		Logger:           slog.Default(),
		CountPreferences: &memoryCountPreferences{scopes: map[int64]string{}},
		Groups:           groups,
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	return &fixture{db: db, svc: svc, bc: bc, chain: chain, staffAccount: staffAccount.ID}
}

func setCountScope(t *testing.T, f *fixture, accountID int64, scope string) {
	t.Helper()
	require.NoError(t, f.svc.SetCountScope(adminCtx(t, accountID), scope))
}

func TestCountScope_DefaultCountsAll(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	startThreadWithQuestions(t, f, f.chain, "Frage 1", "Frage 2")

	scope, err := f.svc.CountScope(adminCtx(t, f.staffAccount))
	require.NoError(t, err)
	assert.Equal(t, messaging.CountScopeAll, scope)
	assert.Equal(t, 2, readUnreadView(t, f, f.staffAccount).badge)
}

// TestCountScope_NoneIsPersonal: "Keine Zahl" darkens the own counter only.
// The inbox keeps its unread rows, and a colleague keeps the full count.
func TestCountScope_NoneIsPersonal(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID, _ := startThreadWithQuestions(t, f, f.chain, "Frage 1", "Frage 2")

	setCountScope(t, f, f.staffAccount, messaging.CountScopeNone)

	own := readUnreadView(t, f, f.staffAccount, f.chain.StudentID)
	assert.Zero(t, own.badge, "own counter")
	assert.Equal(t, map[int64]int{threadID: 2}, own.inbox, "the inbox still shows the conversation as unread")
	assert.Equal(t, map[int64]int{threadID: 2}, own.onlyUnread, "the unread filter still finds it")
	assert.Equal(t, map[int64]int{threadID: 2}, own.cards, "the child card still shows it")

	assert.Equal(t, 2, readUnreadView(t, f, colleague.ID).badge, "a colleague's counter is unchanged")
}

// TestCountScope_NoneSkipsTeamMark: the choice also covers conversations a
// colleague marked unread for the team (#3654).
func TestCountScope_NoneSkipsTeamMark(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, colleague.ID), threadID))

	setCountScope(t, f, f.staffAccount, messaging.CountScopeNone)
	view := readUnreadView(t, f, f.staffAccount)
	assert.Zero(t, view.badge)
	assert.Equal(t, map[int64]int{threadID: 1}, view.onlyUnread, "the mark stays visible in the inbox")
}

// TestCountScope_OwnGroupsCountsOnlyOwnChildren: messages about a child of
// another group, and a team mark on it, stay out of the own counter; the
// same for a child without a group.
func TestCountScope_OwnGroupsCountsOnlyOwnChildren(t *testing.T) {
	t.Parallel()

	groups := groupsByAccount{}
	f := newCountScopeFixture(t, groups)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	ownGroup := testpkg.CreateTestEducationGroup(t, f.db, "Eulen")
	otherGroup := testpkg.CreateTestEducationGroup(t, f.db, "Füchse")
	other := testpkg.CreateTestParentGuardianChain(t, f.db)
	ungrouped := testpkg.CreateTestParentGuardianChain(t, f.db)
	testpkg.AssignStudentToGroup(t, f.db, f.chain.StudentID, ownGroup.ID)
	testpkg.AssignStudentToGroup(t, f.db, other.StudentID, otherGroup.ID)
	groups[f.staffAccount] = []int64{ownGroup.ID}
	groups[colleague.ID] = []int64{otherGroup.ID}

	startThreadWithQuestions(t, f, f.chain, "Frage 1", "Frage 2")
	otherThread, _ := startThreadWithQuestions(t, f, other, "Frage 3")
	startThreadWithQuestions(t, f, ungrouped, "Frage 4")
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, colleague.ID), otherThread))

	hasOwnGroups, err := f.svc.HasOwnGroups(adminCtx(t, f.staffAccount))
	require.NoError(t, err)
	assert.True(t, hasOwnGroups)

	setCountScope(t, f, f.staffAccount, messaging.CountScopeOwnGroups)
	own := readUnreadView(t, f, f.staffAccount)
	assert.Equal(t, 2, own.badge, "only the two messages about the own group's child")
	assert.Len(t, own.onlyUnread, 3, "the inbox keeps every unread conversation")

	setCountScope(t, f, colleague.ID, messaging.CountScopeOwnGroups)
	assert.Equal(t, 1, readUnreadView(t, f, colleague.ID).badge, "the colleague counts the other group's message")

	setCountScope(t, f, f.staffAccount, messaging.CountScopeAll)
	assert.Equal(t, 4, readUnreadView(t, f, f.staffAccount).badge, "back to all counts every message again")
}

// TestCountScope_OwnGroupsCountsTeamMarkOfOwnChild: a team mark on a child of
// the own group lifts the counter like it does under "all".
func TestCountScope_OwnGroupsCountsTeamMarkOfOwnChild(t *testing.T) {
	t.Parallel()

	groups := groupsByAccount{}
	f := newCountScopeFixture(t, groups)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	ownGroup := testpkg.CreateTestEducationGroup(t, f.db, "Eulen")
	testpkg.AssignStudentToGroup(t, f.db, f.chain.StudentID, ownGroup.ID)
	groups[f.staffAccount] = []int64{ownGroup.ID}
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, colleague.ID), threadID))

	setCountScope(t, f, f.staffAccount, messaging.CountScopeOwnGroups)
	assert.Equal(t, 1, readUnreadView(t, f, f.staffAccount).badge)
}

// TestCountScope_OwnGroupsWithoutGroupCountsNothing: an account without an
// OGS group has no own children to count.
func TestCountScope_OwnGroupsWithoutGroupCountsNothing(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	ownGroup := testpkg.CreateTestEducationGroup(t, f.db, "Eulen")
	testpkg.AssignStudentToGroup(t, f.db, f.chain.StudentID, ownGroup.ID)
	startThreadWithQuestions(t, f, f.chain, "Frage")

	setCountScope(t, f, f.staffAccount, messaging.CountScopeOwnGroups)
	assert.Zero(t, readUnreadView(t, f, f.staffAccount).badge)

	hasOwnGroups, err := f.svc.HasOwnGroups(adminCtx(t, f.staffAccount))
	require.NoError(t, err)
	assert.False(t, hasOwnGroups, "the setting can say why the counter stays empty")
}

// TestCountScope_MarkAllReadReportsScopedCount: the count "Alle als gelesen
// markieren" reports back is the own counter, so it follows the scope too.
func TestCountScope_MarkAllReadReportsScopedCount(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, colleague.ID), threadID))

	assert.Equal(t, 1, markAllRead(t, f, adminCtx(t, f.staffAccount)), "the team mark remains under all")
	setCountScope(t, f, f.staffAccount, messaging.CountScopeNone)
	assert.Zero(t, markAllRead(t, f, adminCtx(t, f.staffAccount)), "nothing remains to count under none")
}

func TestCountScope_RejectsUnknownScope(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	for _, scope := range []string{"", "ALL", "mine", "own-groups"} {
		err := f.svc.SetCountScope(adminCtx(t, f.staffAccount), scope)
		require.ErrorIs(t, err, messaging.ErrInvalidCountScope, "scope %q", scope)
	}
	scope, err := f.svc.CountScope(adminCtx(t, f.staffAccount))
	require.NoError(t, err)
	assert.Equal(t, messaging.CountScopeAll, scope, "a rejected value stores nothing")
}

// TestCountScope_DisabledSchoolCountsNothing: a school with messaging off
// keeps the counter dark whatever the scope.
func TestCountScope_DisabledSchoolCountsNothing(t *testing.T) {
	t.Parallel()

	f := newCountScopeFixture(t, groupsByAccount{})
	startThreadWithQuestions(t, f, f.chain, "Frage")
	setCountScope(t, f, f.staffAccount, messaging.CountScopeAll)

	f.svc.Settings = stubSettings{messagingEnabled: false}
	assert.Zero(t, readUnreadView(t, f, f.staffAccount).badge)
}
