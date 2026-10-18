package email

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithInteractiveRetrySetsThePolicyOfAWaitedForMail(t *testing.T) {
	t.Parallel()
	request := WithInteractiveRetry(DeliveryRequest{MaxAttempts: 9}, nil)
	assert.Equal(t, 3, request.MaxAttempts)
	assert.Equal(t, []time.Duration{time.Second, 5 * time.Second, 15 * time.Second}, request.BackoffPolicy)

	request.BackoffPolicy[0] = time.Hour
	assert.Equal(t, time.Second, WithInteractiveRetry(DeliveryRequest{}, nil).BackoffPolicy[0], "callers cannot change the shared spacing")

	custom := []time.Duration{time.Millisecond}
	assert.Equal(t, custom, WithInteractiveRetry(DeliveryRequest{}, custom).BackoffPolicy, "tests replace the spacing")
}
