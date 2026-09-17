package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvitationTokenModelMapsRoleAndCreator(t *testing.T) {
	t.Parallel()
	createdBy := int64(44)
	token := invitationTokenModel(InvitationRecord{
		ID: 7, TenantID: 3, Email: "ada@example.com", RoleID: 9, RoleName: "admin",
		CreatedBy: &createdBy, CreatorEmail: "creator@example.com",
	})
	require.NotNil(t, token.Role)
	assert.Equal(t, "admin", token.Role.Name)
	assert.Equal(t, int64(9), token.Role.ID)
	require.NotNil(t, token.Creator)
	assert.Equal(t, "creator@example.com", token.Creator.Email)
	assert.Equal(t, createdBy, token.Creator.ID)
}
