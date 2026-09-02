package notifications_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

// A decision made in the school portal (#2208) lands on the shared
// (account, type) row, and only the types the school portal offers can be
// decided from there.
func TestSetForPortalAccountHonoursTheSchoolCatalogue(t *testing.T) {
	t.Parallel()
	repo := &fakePreferenceRepo{}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

	require.NoError(t, svc.SetForPortalAccount(context.Background(), prefAccountA, notifications.PortalSchool, notifications.TypeStaffMessage, true))
	require.Len(t, repo.upserted, 1)
	assert.Equal(t, notifications.TypeStaffMessage, repo.upserted[0].NotificationType)
	assert.Equal(t, prefAccountA, repo.upserted[0].AccountID)

	err := svc.SetForPortalAccount(context.Background(), prefAccountA, notifications.PortalSchool, notifications.TypePickupUpcoming, true)
	require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)

	require.Error(t, svc.SetForPortalAccount(context.Background(), prefAccountA, notifications.PortalParent, notifications.TypeParentAnnouncement, true),
		"the parent portal decides through its own path")
}
