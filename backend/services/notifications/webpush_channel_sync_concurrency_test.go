package notifications

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebPushSynchronousDispatchBoundsWorkers(t *testing.T) {
	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var startedOnce sync.Once
	sender := &fakeSender{sendFn: func(context.Context) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		if inFlight == maxConcurrentPushSends {
			startedOnce.Do(func() { started <- struct{}{} })
		}
		mu.Unlock()

		<-release

		mu.Lock()
		inFlight--
		mu.Unlock()
	}}
	channel := testChannel(&fakePushRepo{}, sender)
	subs := make([]*iot.PushSubscription, 0, maxConcurrentPushSends+3)
	for id := range maxConcurrentPushSends + 3 {
		subs = append(subs, testSub(int64(id+1), 41, fmt.Sprintf("https://fcm.googleapis.com/%d", id)))
	}

	done := make(chan error, 1)
	go func() {
		done <- channel.sendAllSynchronously(context.Background(), Event{Type: "test"}, []byte(`{}`), subs)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("synchronous dispatch did not start the bounded worker set")
	}
	mu.Lock()
	assert.Equal(t, maxConcurrentPushSends, maxInFlight)
	mu.Unlock()

	close(release)
	require.NoError(t, <-done)
}
