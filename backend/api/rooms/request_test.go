package rooms

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoomRequestBindCapacity(t *testing.T) {
	request := httptest.NewRequest("POST", "/rooms", nil)

	t.Run("allows an omitted capacity", func(t *testing.T) {
		require.NoError(t, (&RoomRequest{Name: "Mensa"}).Bind(request))
	})

	t.Run("allows a positive capacity", func(t *testing.T) {
		capacity := 43
		require.NoError(t, (&RoomRequest{Name: "Mensa", Capacity: &capacity}).Bind(request))
	})

	t.Run("rejects zero capacity", func(t *testing.T) {
		capacity := 0
		require.Error(t, (&RoomRequest{Name: "Mensa", Capacity: &capacity}).Bind(request))
	})
}
