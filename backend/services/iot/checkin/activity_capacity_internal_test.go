package checkin

import (
	"context"
	"testing"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/require"
)

func TestCheckActivityCapacitySkipsUnlimitedActivity(t *testing.T) {
	t.Parallel()

	service := &CheckinService{}
	group := &activeModel.Group{
		ActualGroup: &activitiesModel.Group{
			Name:            "Sporthalle",
			MaxParticipants: 0,
		},
	}

	require.NoError(t, service.checkActivityCapacity(context.Background(), group))
}
