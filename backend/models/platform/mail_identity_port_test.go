package platform_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	platform "github.com/moto-nrw/project-phoenix/models/platform"
)

type stubResolver struct {
	identity platform.TenantMailIdentity
	err      error
}

func (s stubResolver) ResolveTenantMailIdentity(
	_ context.Context,
	_ int64,
) (platform.TenantMailIdentity, error) {
	return s.identity, s.err
}

// A failed lookup must degrade to "no reply address" rather than surfacing an
// error every send site would have to handle — losing the return path must
// never cost the mail (#1936).
func TestResolveReplyToIdentity_ErrorDegradesToZero(t *testing.T) {
	t.Parallel()

	got := platform.ResolveReplyToIdentity(
		context.Background(),
		stubResolver{err: errors.New("boom")},
		42,
		nil,
	)
	assert.True(t, got.IsZero())
}

// No resolver wired (unit tests, CLI paths) behaves exactly as before.
func TestResolveReplyToIdentity_NilResolverIsZero(t *testing.T) {
	t.Parallel()

	assert.True(t, platform.ResolveReplyToIdentity(context.Background(), nil, 42, nil).IsZero())
}

func TestResolveReplyToIdentity_PassesIdentityThrough(t *testing.T) {
	t.Parallel()

	got := platform.ResolveReplyToIdentity(
		context.Background(),
		stubResolver{identity: platform.TenantMailIdentity{
			ReplyToName:    "OGS Am Berg",
			ReplyToAddress: "ogs@schule.example",
		}},
		42,
		nil,
	)
	assert.Equal(t, "ogs@schule.example", got.ReplyToAddress)
	assert.Equal(t, "OGS Am Berg", got.ReplyToName)
	assert.False(t, got.IsZero())
}
