package identityaccess_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// A parents-portal removal runs as a parent and never clears the payer.
func TestParentRevokeAccessRequest(t *testing.T) {
	t.Parallel()

	studentID, profileID, actorID := int64(4082), int64(2580), int64(3332)
	assert.Equal(t, identityaccess.RevokeAccessRequest{
		StudentID: studentID, GuardianProfileID: profileID, ActorAccountID: actorID, ByParent: true, MayClearPayer: false,
	}, identityaccess.ParentRevokeAccessRequest(studentID, profileID, actorID))
}
