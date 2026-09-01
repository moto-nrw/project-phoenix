package compose

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestProviderResultMapsPublicCancellationToDomainDecision(t *testing.T) {
	t.Parallel()
	_, err := providerResult(delivery.ProviderResult{}, fmt.Errorf("access revoked: %w", delivery.ErrCancelled))
	assert.True(t, errors.Is(err, domain.ErrCancelled))
}
