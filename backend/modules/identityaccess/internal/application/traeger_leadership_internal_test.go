package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// TestTraegerLeadershipRules pins the hierarchy of #5471 apart from the
// role table: only a person with Leitung grants, withdraws or removes
// Leitung, even should another role ever grant "Konten verwalten", and the
// last usable Leitung stays, also against itself.
func TestTraegerLeadershipRules(t *testing.T) {
	t.Parallel()
	const leadership, administration = identityaccess.TraegerRoleLeadership, identityaccess.TraegerRoleAdministration
	members := []identityPorts.TraegerMembership{{AccountID: 11}, {AccountID: 12}}

	other := traegerUnit{members: members, leaders: []int64{11}, actorRoles: []string{administration}}
	assert.ErrorIs(t, other.requireLeadershipFor(nil, []string{leadership}), identityaccess.ErrTraegerAccountsForbidden, "grant")
	assert.ErrorIs(t, other.requireLeadershipFor([]string{leadership}, []string{administration}), identityaccess.ErrTraegerAccountsForbidden, "withdraw")
	assert.ErrorIs(t, other.requireLeadershipFor([]string{leadership}, nil), identityaccess.ErrTraegerAccountsForbidden, "remove")
	assert.NoError(t, other.requireLeadershipFor([]string{administration}, nil), "a change without Leitung needs no Leitung")
	assert.NoError(t, other.requireLeadershipFor([]string{leadership}, []string{leadership, administration}), "Leitung kept")

	leader := traegerUnit{members: members, leaders: []int64{11}, actorRoles: []string{leadership}}
	assert.NoError(t, leader.requireLeadershipFor(nil, []string{leadership}))
	assert.True(t, leavesNoLeader(leader, 11, []string{leadership}, []string{administration}), "the last Leitung cannot give itself away")
	assert.True(t, leavesNoLeader(leader, 11, []string{leadership}, nil), "nor be removed")
	assert.False(t, leavesNoLeader(leader, 11, []string{leadership}, []string{leadership}))

	two := traegerUnit{members: members, leaders: []int64{11, 12}, actorRoles: []string{leadership}}
	assert.False(t, leavesNoLeader(two, 11, []string{leadership}, nil), "with another usable Leitung the first may go")

	gone := traegerUnit{members: members[:1], leaders: []int64{11, 12}, actorRoles: []string{leadership}}
	assert.True(t, leavesNoLeader(gone, 11, []string{leadership}, nil), "a Leitung without an active membership does not count")
}
