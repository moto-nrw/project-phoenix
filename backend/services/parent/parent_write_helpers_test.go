package parent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
)

func TestChildHasPermission(t *testing.T) {
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
	svc := &service{}

	_, err := svc.resolvePermittedChild(context.Background(), 0, 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id")

	_, err = svc.resolvePermittedChild(context.Background(), 1, 0, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "student_id")
}
