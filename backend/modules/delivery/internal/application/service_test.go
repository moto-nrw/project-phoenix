package application

import (
	"context"
	"errors"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type guardianDirectory struct {
	displays []deliveryContract.GuardianDisplay
	err      error
}

func (d guardianDirectory) ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error) {
	return d.displays, d.err
}

func TestEmailDeliveryStatusesFailsWhenGuardianDisplayLookupFails(t *testing.T) {
	t.Parallel()
	profileID := int64(51)
	store := &workerStore{statuses: []deliveryContract.EmailDeliveryStatus{{DeliveryID: 7, GuardianProfileID: &profileID}}}
	lookupErr := errors.New("people directory unavailable")
	service := NewService(store, guardianDirectory{err: lookupErr}, func(domain.Observation) {})

	_, err := service.EmailDeliveryStatuses(context.Background(), 3, deliveryContract.RelatedEntity{Type: "announcement", ID: 9})

	require.Error(t, err)
	assert.ErrorIs(t, err, lookupErr)
}

func TestEmailDeliveryStatusesAddsNamesAndSorts(t *testing.T) {
	t.Parallel()
	firstID, secondID := int64(51), int64(52)
	store := &workerStore{statuses: []deliveryContract.EmailDeliveryStatus{
		{DeliveryID: 8, GuardianProfileID: &secondID},
		{DeliveryID: 7, GuardianProfileID: &firstID},
	}}
	service := NewService(store, guardianDirectory{displays: []deliveryContract.GuardianDisplay{
		{GuardianProfileID: firstID, FirstName: "Ada", LastName: "A"},
		{GuardianProfileID: secondID, FirstName: "Berta", LastName: "B"},
	}}, func(domain.Observation) {})

	rows, err := service.EmailDeliveryStatuses(context.Background(), 3, deliveryContract.RelatedEntity{Type: "announcement", ID: 9})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Ada", rows[0].FirstName)
	assert.Equal(t, "Berta", rows[1].FirstName)
}
