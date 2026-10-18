package identityaccess_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The operator portal's invitation is an operator grant and carries the
// operator's input unchanged.
func TestOperatorSchoolInvitationIsAnOperatorGrant(t *testing.T) {
	t.Parallel()
	roleID, tenantID := int64(4082), int64(2580)
	first, last, position := "Ada", "Lovelace", "Leitung"
	assert.Equal(t, identityaccess.SchoolInvitationRequest{
		Email: "ada@example.org", RoleID: roleID, TenantID: tenantID,
		FirstName: &first, LastName: &last, Position: &position,
		CaregiverEnabled: true, OperatorGrant: true,
	}, identityaccess.OperatorSchoolInvitation("ada@example.org", roleID, tenantID, &first, &last, &position, true))
}
