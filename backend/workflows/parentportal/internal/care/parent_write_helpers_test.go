package care

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChildHasPermission(t *testing.T) {
	t.Parallel()

	child := &ChildSummary{
		GuardianPermissions: map[string]bool{
			GuardianPermissionPortalAccess: true,
			GuardianPermissionNotesWrite:   false,
		},
	}

	assert.True(t, childHasPermission(child, GuardianPermissionPortalAccess))
	assert.False(t, childHasPermission(child, GuardianPermissionNotesWrite))
	assert.False(t, childHasPermission(child, GuardianPermissionSickNoteSubmit))
	assert.False(t, childHasPermission(nil, GuardianPermissionPortalAccess))
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

	permissions := map[string]bool{
		GuardianPermissionMasterDataEdit: true,
	}

	child := &ChildSummary{GuardianPermissions: permissions}
	assert.True(t, childHasPermission(child, GuardianPermissionMasterDataEdit))
	assert.False(t, childHasPermission(child, GuardianPermissionNotesWrite))
	assert.False(t, childHasPermission(nil, GuardianPermissionMasterDataEdit))

	resolved := &Child{GuardianPermissions: permissions}
	assert.True(t, resolved.HasPermission(GuardianPermissionMasterDataEdit))
	assert.False(t, resolved.HasPermission(GuardianPermissionNotesWrite))
	assert.False(t, (*Child)(nil).HasPermission(GuardianPermissionMasterDataEdit))
}
