package application

import (
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoalesceFeedCreationSharesConcurrentResult(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := &service{}
		var calls atomic.Int32
		release := make(chan struct{})
		create := func() (string, string, error) {
			calls.Add(1)
			// The first call stays open until the second caller has joined it.
			<-release
			return "https://moto.test/api/calendar-feed/token", "webcal://moto.test/api/calendar-feed/token", nil
		}

		type result struct {
			httpsURL  string
			webcalURL string
			err       error
		}
		results := make(chan result, 2)
		for range 2 {
			go func() {
				httpsURL, webcalURL, err := service.coalesceFeedCreation("staff:41:73", create)
				results <- result{httpsURL: httpsURL, webcalURL: webcalURL, err: err}
			}()
		}

		// The leader is blocked in create and the other caller in the shared
		// flight. Wait establishes that overlap without sleeps.
		synctest.Wait()
		assert.Equal(t, int32(1), calls.Load(), "only the leader may create the feed")
		close(release)

		first := <-results
		second := <-results
		require.NoError(t, first.err)
		require.NoError(t, second.err)
		assert.NotEmpty(t, first.httpsURL)
		assert.Equal(t, first.httpsURL, second.httpsURL)
		assert.Equal(t, first.webcalURL, second.webcalURL)
		assert.Equal(t, int32(1), calls.Load())
	})
}
