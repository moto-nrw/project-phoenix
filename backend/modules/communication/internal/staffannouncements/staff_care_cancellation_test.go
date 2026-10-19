package announcement_test

import (
	"context"
	"log/slog"
	"testing"

	records "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/roots/communicationtest"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// noticeSettings keeps the dispatch switch on so the push router does not
// short-circuit. The notice itself has no school settings since #3731.
func noticeSettings() *settingsDouble {
	values := map[string]bool{
		configModel.KeyNotificationsDispatchEnabled: true,
	}
	return &settingsDouble{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return values[key], nil
		},
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			return values[key], nil
		},
	}
}

func buildNoticeService(t *testing.T, settings *settingsDouble) (announcement.StaffService, *bun.DB, *api.ParentMessagingSuiteRepositories) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	svc := announcement.NewService(announcement.ServiceConfig{
		Repo:       records.NewRepository(reposMessages.ParentAnnouncement),
		Settings:   configModel.NewParentAnnouncementValues(settings),
		Notifier:   noticeNotificationCallback(owners.BindCommunicationNotificationSender(api.NewCommunicationTestNotifier(settings, slog.Default()))),
		ParentsURL: "https://parents.example.test",
		Logger:     slog.Default(),
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.NewParentDeliveryPolicy())
	return svc, db, reposMessages
}

func TestPublishCareCancellation_PublishesSystemRowForBookedChildren(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeService(t, noticeSettings())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)

	var result *announcement.CareCancellationResult
	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, ctx, db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		result, err = testpkg.TenantQuery1(t, txCtx, db, svc.PublishCareCancellation, announcement.CareCancellationInput{
			StudentIDs: []int64{chain.StudentID, chain.StudentID},
			Title:      "Fußball-AG am Dienstag entfällt",
			Body:       "Die Fußball-AG am Dienstag von 14:00 bis 15:30 fällt aus.",
			CreatedBy:  chain.AccountID,
		})
		return err
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.RecipientCount, "the one linked guardian is reached")

	var stored *announcement.StaffParentAnnouncement
	require.NoError(t, tenant.WithTenantTx(testpkg.WithTenantRuntime(t, ctx, db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		stored, err = records.NewRepository(repos.ParentAnnouncement).FindByID(txCtx, result.Announcement.ID)
		if err != nil {
			return err
		}
		targets, err := records.NewRepository(repos.ParentAnnouncement).ListTargets(txCtx, stored.ID)
		if err != nil {
			return err
		}
		stored.Targets = targets
		return nil
	}))
	require.NotNil(t, stored)
	require.NotNil(t, stored.SystemKind)
	assert.Equal(t, announcement.StaffParentAnnouncementSystemKindCareCancellation, *stored.SystemKind)
	assert.True(t, stored.IsPublished(), "the notice is live immediately")
	assert.Equal(t, announcement.StaffParentAnnouncementPriorityImportant, stored.Priority)
	assert.True(t, stored.SendEmail, "the dialog promises App und E-Mail; family opt-outs apply in the outbox")
	require.Len(t, stored.Targets, 1, "duplicate student ids collapse to one target")
	assert.Equal(t, announcement.StaffAnnouncementTargetStudent, stored.Targets[0].TargetType)
	require.NotNil(t, stored.Targets[0].TargetRefID)
	assert.Equal(t, chain.StudentID, *stored.Targets[0].TargetRefID)
}

func TestPublishCareCancellation_RejectsEmptyText(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, noticeSettings())
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := testpkg.TenantQuery1(t, txCtx, db, svc.PublishCareCancellation, announcement.CareCancellationInput{
			StudentIDs: []int64{chain.StudentID},
			Title:      "   ",
			Body:       "Heute keine Betreuung.",
			CreatedBy:  chain.AccountID,
		})
		return err
	})
	assert.ErrorIs(t, err, announcement.ErrValidation)
}

func TestCareCancellationReachFor_CountsLinkedGuardians(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, noticeSettings())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	// A second child with no guardian at all must not inflate the count.
	orphan := testpkg.CreateTestStudent(t, db, "Ohne", "Eltern", "1a")

	var reach *announcement.CareCancellationReach
	require.NoError(t, tenant.WithTenantTx(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		reach, err = testpkg.TenantQuery1(t, txCtx, db, svc.CareCancellationReachFor, []int64{chain.StudentID, orphan.ID})
		return err
	}))
	require.NotNil(t, reach)
	assert.Equal(t, 1, reach.FamilyCount)
}

// noticeNotificationCallback observes the same real router through its native
// public event boundary; it does not replace consent or audience policy.
type noticeNotificationCallback func(context.Context, communication.NotificationEvent) error

func (send noticeNotificationCallback) Notify(ctx context.Context, event communication.NotificationEvent) error {
	return send(ctx, event)
}
