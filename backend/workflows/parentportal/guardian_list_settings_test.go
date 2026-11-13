package parentportal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/require"
)

func TestListChildGuardiansSettingsFailurePrecedesPayload(t *testing.T) {
	t.Parallel()
	svc, db := buildGuardianService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	cause := errors.New("guardian settings unavailable")
	svc.ChildFlows.Settings = parentSettingsStub{boolErr: cause}
	// The payload must not be read when the preceding setting fails.
	svc.ChildFlows.GuardianProfileRepo = nil

	rows, err := svc.ListChildGuardians(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.Nil(t, rows)
	require.ErrorIs(t, err, cause)
	require.EqualError(t, err, "parent: resolve guardian-management setting: guardian settings unavailable")
}

func TestListChildGuardiansRealSettingDefaultAndOverrides(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		override bool
		enabled  bool
	}{
		{name: "registry default", enabled: true},
		{name: "disabled override", override: true, enabled: false},
		{name: "override equal to default", override: true, enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			svc, db := buildGuardianService(t)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			contactID := linkContactOnlyGuardian(t, db, chain.StudentID, "guardian-setting")
			settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			if tc.override {
				require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), parentService.KeyParentGuardianManagementEnabled, tc.enabled, nil, nil))
			}
			svc.ChildFlows.Settings = settings.Settings

			rows, err := svc.ListChildGuardians(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
			require.NoError(t, err)
			require.Len(t, rows, 2)
			byID := map[int64]*parentService.ChildGuardian{}
			for _, row := range rows {
				byID[row.GuardianProfileID] = row
			}
			self := byID[chain.GuardianProfileID]
			contact := byID[contactID]
			require.NotNil(t, self)
			require.NotNil(t, contact)
			require.True(t, self.IsSelf)
			require.Equal(t, chain.Email, self.Email)
			require.Equal(t, tc.enabled, self.CanEditContact)
			require.False(t, self.CanManagePickup)
			require.Equal(t, tc.enabled, contact.CanEditContact)
			require.Equal(t, tc.enabled, contact.CanManagePickup)
		})
	}
}
