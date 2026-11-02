package compose_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

// Exercise the queued producer and the same wire receiver used by the second API.
func TestArrivalScheduleSignalReachesOnlyItsSchoolAcrossAPIs(t *testing.T) {
	t.Parallel()
	api2, err := realtime.NewRelayedHub(nil, func(string) error { t.Error("received signal was republished"); return nil }, nil)
	require.NoError(t, err)
	schoolA := &realtime.Client{Channel: make(chan realtime.Event, 4)}
	schoolB := &realtime.Client{Channel: make(chan realtime.Event, 4)}
	api2.Register(schoolA, 17, nil)
	api2.Register(schoolB, 29, nil)
	var payload string
	api1, err := realtime.NewRelayedHub(nil, func(s string) error { payload = s; return api2.ReceiveSignal(s) }, nil)
	require.NoError(t, err)
	localA := &realtime.Client{Channel: make(chan realtime.Event, 4)}
	localB := &realtime.Client{Channel: make(chan realtime.Event, 4)}
	api1.Register(localA, 17, nil)
	api1.Register(localB, 29, nil)
	compose.QueueArrivalScheduleChanged(tenant.WithTenantID(context.Background(), 17), api1, nil)
	require.Len(t, schoolA.Channel, 1, "API 2 must receive the school's arrival invalidation")
	require.Equal(t, realtime.EventArrivalScheduleChanged, (<-schoolA.Channel).Type)
	require.Len(t, localA.Channel, 1)
	require.Empty(t, schoolB.Channel)
	require.Empty(t, localB.Channel)
	var wire map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &wire))
	require.Equal(t, float64(17), wire["school_id"])
	require.Equal(t, "tenant", wire["audience"])
}
