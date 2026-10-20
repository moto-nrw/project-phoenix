package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

func TestEmploymentResetRequiresInvitationTransaction(t *testing.T) {
	t.Parallel()
	// No dependency may be touched before the ambient transaction is proven.
	reset := EmploymentResetter{}
	input := workforce.EmploymentReset{StaffID: 1, ActorAccountID: 2, InvitationID: 3, MasterData: true, Qualifications: true, WorkTime: true}
	err := reset.ResetEmployment(context.Background(), input)
	require.ErrorContains(t, err, "requires an invitation transaction")
	_, err = reset.PrepareEmploymentReset(context.Background(), input)
	require.ErrorContains(t, err, "requires an invitation transaction")
}
