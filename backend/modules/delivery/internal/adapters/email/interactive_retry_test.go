package email

import (
	"testing"
	"time"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"

	"github.com/stretchr/testify/assert"
)

func TestWithInteractiveRetrySetsThePolicyOfAWaitedForMail(t *testing.T) {
	t.Parallel()
	request := mailcontract.WithInteractiveRetry(mailcontract.DeliveryRequest{MaxAttempts: 9}, nil)
	assert.Equal(t, 3, request.MaxAttempts)
	assert.Equal(t, []time.Duration{time.Second, 5 * time.Second, 15 * time.Second}, request.BackoffPolicy)

	request.BackoffPolicy[0] = time.Hour
	assert.Equal(t, time.Second, mailcontract.WithInteractiveRetry(mailcontract.DeliveryRequest{}, nil).BackoffPolicy[0], "callers cannot change the shared spacing")

	custom := []time.Duration{time.Millisecond}
	assert.Equal(t, custom, mailcontract.WithInteractiveRetry(mailcontract.DeliveryRequest{}, custom).BackoffPolicy, "tests replace the spacing")
}
