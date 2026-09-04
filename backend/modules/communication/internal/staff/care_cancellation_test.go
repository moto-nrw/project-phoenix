package announcement_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/internal/staff"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// noticeSettings answers the three cancellation-notice keys and keeps the
// dispatch switch on so the push router does not short-circuit.
func noticeSettings(enabled, email bool) *configtest.Mock {
	values := map[string]bool{
		configModel.KeyNotificationsCareCancelledEnabled:   enabled,
		configModel.KeyNotificationsCareCancelledDefaultOn: true,
		configModel.KeyNotificationsCareCancelledEmail:     email,
		configModel.KeyNotificationsDispatchEnabled:        true,
	}
	return &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return values[key], nil
		},
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			return values[key], nil
		},
	}
}

func buildNoticeService(t *testing.T, settings *configtest.Mock) (announcement.Service, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := announcement.NewService(announcement.ServiceConfig{
		Repo:       repos.ParentAnnouncement,
		Settings:   settings,
		Notifier:   notifications.NewService(settings, slog.Default()),
		ParentsURL: "https://parents.example.test",
		Logger:     slog.Default(),
	})
	return svc, db, repos
}

func TestPublishCareCancellation_PublishesSystemRowForBookedChildren(t *testing.T) {
	t.Parallel()
	svc, db, repos := buildNoticeService(t, noticeSettings(true, false))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)

	var result *announcement.CareCancellationResult
	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, ctx, db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		result, err = svc.PublishCareCancellation(txCtx, announcement.CareCancellationInput{
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

	var stored *usersModels.ParentAnnouncement
	require.NoError(t, tenant.WithTenantTx(testpkg.WithTenantRuntime(t, ctx, db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		stored, err = repos.ParentAnnouncement.FindByID(txCtx, result.Announcement.ID)
		if err != nil {
			return err
		}
		targets, err := repos.ParentAnnouncement.ListTargets(txCtx, stored.ID)
		if err != nil {
			return err
		}
		stored.Targets = targets
		return nil
	}))
	require.NotNil(t, stored)
	require.NotNil(t, stored.SystemKind)
	assert.Equal(t, usersModels.ParentAnnouncementSystemKindCareCancellation, *stored.SystemKind)
	assert.True(t, stored.IsPublished(), "the notice is live immediately")
	assert.Equal(t, usersModels.ParentAnnouncementPriorityImportant, stored.Priority)
	assert.False(t, stored.SendEmail, "e-mail follows the school setting")
	require.Len(t, stored.Targets, 1, "duplicate student ids collapse to one target")
	assert.Equal(t, usersModels.AnnouncementTargetStudent, stored.Targets[0].TargetType)
	require.NotNil(t, stored.Targets[0].TargetRefID)
	assert.Equal(t, chain.StudentID, *stored.Targets[0].TargetRefID)
}

func TestPublishCareCancellation_RefusesWhenSchoolSwitchedItOff(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, noticeSettings(false, false))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := svc.PublishCareCancellation(txCtx, announcement.CareCancellationInput{
			StudentIDs: []int64{chain.StudentID},
			Title:      "Entfällt",
			Body:       "Heute keine Betreuung.",
			CreatedBy:  chain.AccountID,
		})
		return err
	})
	assert.ErrorIs(t, err, announcement.ErrCareCancellationDisabled)
}

func TestPublishCareCancellation_RejectsEmptyText(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, noticeSettings(true, false))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := svc.PublishCareCancellation(txCtx, announcement.CareCancellationInput{
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
	svc, db, _ := buildNoticeService(t, noticeSettings(true, false))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	// A second child with no guardian at all must not inflate the count.
	orphan := testpkg.CreateTestStudent(t, db, "Ohne", "Eltern", "1a")

	var reach *announcement.CareCancellationReach
	require.NoError(t, tenant.WithTenantTx(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		reach, err = svc.CareCancellationReachFor(txCtx, []int64{chain.StudentID, orphan.ID})
		return err
	}))
	require.NotNil(t, reach)
	assert.True(t, reach.Enabled)
	assert.True(t, reach.DefaultOn)
	assert.Equal(t, 1, reach.FamilyCount)
}

func TestCareCancellationReachFor_DisabledSchoolReportsNoReach(t *testing.T) {
	t.Parallel()
	svc, db, _ := buildNoticeService(t, noticeSettings(false, false))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	var reach *announcement.CareCancellationReach
	require.NoError(t, tenant.WithTenantTx(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db), db, chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		reach, err = svc.CareCancellationReachFor(txCtx, []int64{chain.StudentID})
		return err
	}))
	require.NotNil(t, reach)
	assert.False(t, reach.Enabled)
	assert.Zero(t, reach.FamilyCount)
}
