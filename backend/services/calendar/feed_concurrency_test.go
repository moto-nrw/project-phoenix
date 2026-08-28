package calendar

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoalesceFeedCreationSharesConcurrentResult(t *testing.T) {
	t.Parallel()

	service := &service{}
	var calls atomic.Int32
	create := func() (string, string, error) {
		calls.Add(1)
		// Keep the first call open long enough for the second caller to join it.
		time.Sleep(250 * time.Millisecond)
		return "https://moto.test/api/calendar-feed/token", "webcal://moto.test/api/calendar-feed/token", nil
	}

	type result struct {
		httpsURL  string
		webcalURL string
		err       error
	}
	start := make(chan struct{})
	ready := sync.WaitGroup{}
	ready.Add(2)
	results := make(chan result, 2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			httpsURL, webcalURL, err := service.coalesceFeedCreation("staff:41:73", create)
			results <- result{httpsURL: httpsURL, webcalURL: webcalURL, err: err}
		}()
	}
	ready.Wait()
	close(start)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.NotEmpty(t, first.httpsURL)
	assert.Equal(t, first.httpsURL, second.httpsURL)
	assert.Equal(t, first.webcalURL, second.webcalURL)
	assert.Equal(t, int32(1), calls.Load())
}
