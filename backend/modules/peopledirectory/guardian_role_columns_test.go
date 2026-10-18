package peopledirectory_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
)

func TestGuardianRoleColumnsKeepTheOtherRelationshipFields(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"relationship_type", "is_emergency_contact", "can_pickup", "emergency_priority", "guardian_role", "permissions"},
		peopledirectory.GuardianRoleColumns())
}
