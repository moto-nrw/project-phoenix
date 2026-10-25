package presenceprojectionhttp

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestRouteRegistrationRequiresTenantSettings(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { NewResource(ResourceConfig{}).Register(chi.NewRouter(), nil) })
}
