package care_test

// Parent side of the leadership conversations (#5200): the choice exists only
// while the school offers it and somebody holds the permission; a started
// leadership conversation stays writable after the school switched it off;
// group-thread answers always go to the team.

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// switchableLeadership is the school's leadership option, which a test can
// switch off between two requests.
type switchableLeadership struct {
	enabled atomic.Bool
	names   []string
}

func (l *switchableLeadership) option(context.Context, bool) (parentportal.LeadershipOption, error) {
	return parentportal.LeadershipOption{Enabled: l.enabled.Load(), Names: l.names}, nil
}

func TestParentLeadership_UnavailableWithoutSettingOrLeaders(t *testing.T) {
	t.Parallel()
	for name, leadership := range map[string]*switchableLeadership{
		"setting off": {names: []string{"Swantje M."}},
		"no leaders":  newSwitchableLeadership(true),
	} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			t.Parallel()
			svc, _, db, _ := buildReadServiceWithNotifier(t, true, nil, leadership.option)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ctx := testpkg.WithPackageTenantRuntime(context.Background())

			view, err := svc.GetChildConversation(ctx, chain.AccountID, chain.StudentID, parentportal.MessageAudienceLeadership)
			require.NoError(t, err)
			assert.False(t, view.Leadership.Available(), "the compose box offers no leadership choice")
			assert.Zero(t, view.ThreadID)

			_, err = svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Nur für die Leitung", 0, parentportal.MessageAudienceLeadership)
			require.ErrorIs(t, err, parentportal.ErrLeadershipMessagesUnavailable)
		})
	}
}

func newSwitchableLeadership(enabled bool, names ...string) *switchableLeadership {
	l := &switchableLeadership{names: names}
	l.enabled.Store(enabled)
	return l
}

func TestParentLeadership_SeparateConversationThatOutlivesTheSwitch(t *testing.T) {
	t.Parallel()
	leadership := newSwitchableLeadership(true, "Swantje M.")
	notifier := &recordingStaffParentMessageNotifier{}
	svc, bc, db, _ := buildReadServiceWithNotifier(t, true, notifier, leadership.option)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	team, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "An das ganze Team", 0, "")
	require.NoError(t, err)
	assert.Equal(t, parentportal.MessageAudienceTeam, team.Audience)
	assert.True(t, team.Leadership.Available())
	assert.Equal(t, []string{"Swantje M."}, team.Leadership.Names)
	assert.Empty(t, team.LeadershipReaders, "a team conversation is read by all staff")

	bc.Reset()
	leader, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Gesprächstermin bitte", 0, parentportal.MessageAudienceLeadership)
	require.NoError(t, err)
	assert.NotEqual(t, team.ThreadID, leader.ThreadID, "the leadership has its own conversation")
	assert.Equal(t, parentportal.MessageAudienceLeadership, leader.Audience)
	assert.Equal(t, []string{"Swantje M."}, leader.LeadershipReaders)
	assert.Equal(t, "Sabine Schneider", leader.GuardianName, "the guardian is the only parent who reads it")
	require.Len(t, leader.Messages, 1)
	assert.Empty(t, bc.CallsByMethod("parent"), "no tenant-wide staff wake")
	assert.NotEmpty(t, bc.CallsByMethod("leadership"))
	require.Len(t, notifier.reports, 2)
	assert.False(t, notifier.reports[0].Leadership)
	assert.True(t, notifier.reports[1].Leadership, "the push goes to the leadership only")

	threads, err := svc.ListChildThreads(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	audiences := map[string]int64{}
	for _, thread := range threads {
		audiences[thread.Audience] = thread.ThreadID
	}
	assert.Equal(t, map[string]int64{parentportal.MessageAudienceTeam: team.ThreadID, parentportal.MessageAudienceLeadership: leader.ThreadID}, audiences)

	teamView, err := svc.GetChildConversation(ctx, chain.AccountID, chain.StudentID, "")
	require.NoError(t, err)
	require.Len(t, teamView.Messages, 1, "the leadership message stays out of the team conversation")

	// Switched off: the existing conversation stays readable and writable.
	leadership.enabled.Store(false)
	again, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Noch eine Frage", 0, parentportal.MessageAudienceLeadership)
	require.NoError(t, err)
	assert.Equal(t, leader.ThreadID, again.ThreadID)
	assert.False(t, again.Leadership.Available(), "no new leadership conversation is offered")
	assert.Equal(t, []string{"Swantje M."}, again.LeadershipReaders, "its readers are still named")
}

func TestParentLeadership_RejectsGroupAnswerAndUnknownAudience(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := buildReadServiceWithNotifier(t, true, nil, newSwitchableLeadership(true, "Swantje M.").option)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Antwort", 4711, parentportal.MessageAudienceLeadership)
	require.ErrorIs(t, err, parentportal.ErrInvalidMessageAudience, "a group-thread answer goes to the team")
	_, err = svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Hallo", 0, "everyone")
	require.ErrorIs(t, err, parentportal.ErrInvalidMessageAudience)
	_, err = svc.GetChildConversation(ctx, chain.AccountID, chain.StudentID, "everyone")
	require.ErrorIs(t, err, parentportal.ErrInvalidMessageAudience)
}

// TestParentLeadership_NoNamesWhileOffWithoutConversation: a guardian without
// a leadership conversation learns no leadership names while the school does
// not offer the choice.
func TestParentLeadership_NoNamesWhileOffWithoutConversation(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := buildReadServiceWithNotifier(t, true, nil, newSwitchableLeadership(false, "Swantje M.").option)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	view, err := svc.GetChildConversation(ctx, chain.AccountID, chain.StudentID, parentportal.MessageAudienceLeadership)
	require.NoError(t, err)
	assert.Zero(t, view.ThreadID)
	assert.Empty(t, view.LeadershipReaders, "no names while the choice is off and no conversation exists")
	assert.False(t, view.Leadership.Available())
}

// TestParentLeadership_ClosedOnceNobodyReads: a started leadership
// conversation takes no new message once nobody is left to read it; its
// history stays readable and names no reader.
func TestParentLeadership_ClosedOnceNobodyReads(t *testing.T) {
	t.Parallel()
	leadership := newSwitchableLeadership(true, "Swantje M.")
	svc, _, db, _ := buildReadServiceWithNotifier(t, true, nil, leadership.option)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	started, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Gesprächstermin bitte", 0, parentportal.MessageAudienceLeadership)
	require.NoError(t, err)

	leadership.names = nil
	_, err = svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Noch da?", 0, parentportal.MessageAudienceLeadership)
	require.ErrorIs(t, err, parentportal.ErrLeadershipMessagesUnavailable)

	view, err := svc.GetChildConversation(ctx, chain.AccountID, chain.StudentID, parentportal.MessageAudienceLeadership)
	require.NoError(t, err)
	assert.Equal(t, started.ThreadID, view.ThreadID)
	require.Len(t, view.Messages, 1, "the history stays readable")
	assert.Empty(t, view.LeadershipReaders)
}
