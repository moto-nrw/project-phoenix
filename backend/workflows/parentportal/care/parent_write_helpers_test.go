package care

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
)

func TestChildHasPermission(t *testing.T) {
	t.Parallel()

	child := &parentModels.ChildSummary{
		GuardianPermissions: map[string]interface{}{
			authorize.GuardianPermissionPortalAccess: true,
			authorize.GuardianPermissionNotesWrite:   false,
		},
	}

	assert.True(t, childHasPermission(child, authorize.GuardianPermissionPortalAccess))
	assert.False(t, childHasPermission(child, authorize.GuardianPermissionNotesWrite))
	assert.False(t, childHasPermission(child, authorize.GuardianPermissionSickNoteSubmit))
	assert.False(t, childHasPermission(nil, authorize.GuardianPermissionPortalAccess))
}

func TestResolvePermittedChildRejectsNonPositiveIDs(t *testing.T) {
	t.Parallel()

	svc := &Service{}

	_, err := svc.ResolvePermittedChild(context.Background(), 0, 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id")

	_, err = svc.ResolvePermittedChild(context.Background(), 1, 0, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "student_id")
}

func TestParentWritePermissionHelpers(t *testing.T) {
	t.Parallel()

	permissions := map[string]interface{}{
		authorize.GuardianPermissionMasterDataEdit: true,
	}

	child := &parentModels.ChildSummary{GuardianPermissions: permissions}
	assert.True(t, childHasPermission(child, authorize.GuardianPermissionMasterDataEdit))
	assert.False(t, childHasPermission(child, authorize.GuardianPermissionNotesWrite))
	assert.False(t, childHasPermission(nil, authorize.GuardianPermissionMasterDataEdit))

	resolved := &Child{GuardianPermissions: permissions}
	assert.True(t, resolved.HasPermission(authorize.GuardianPermissionMasterDataEdit))
	assert.False(t, resolved.HasPermission(authorize.GuardianPermissionNotesWrite))
	assert.False(t, (*Child)(nil).HasPermission(authorize.GuardianPermissionMasterDataEdit))
}
