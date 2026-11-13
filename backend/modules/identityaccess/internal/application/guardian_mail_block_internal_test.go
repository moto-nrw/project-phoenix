package application

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestBulkInviteNamesTheMailBlock(t *testing.T) {
	t.Parallel()
	reason, blocked := bulkInviteMailBlock(domain.GuardianMailBlockAddress)
	assert.True(t, blocked)
	assert.Equal(t, domain.BulkInviteProblemAddressBlocked, reason)
	reason, blocked = bulkInviteMailBlock(domain.GuardianMailBlockDelayed)
	assert.True(t, blocked)
	assert.Equal(t, domain.BulkInviteProblemMailDelayed, reason)
	_, blocked = bulkInviteMailBlock("")
	assert.False(t, blocked)
}
