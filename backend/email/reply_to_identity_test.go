package email

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubReplyToResolver struct {
	identity ReplyToIdentity
	err      error
}

func (s stubReplyToResolver) ResolveReplyTo(context.Context, int64) (ReplyToIdentity, error) {
	return s.identity, s.err
}

func TestResolveReplyToIdentity(t *testing.T) {
	t.Parallel()

	t.Run("resolver error degrades to zero", func(t *testing.T) {
		t.Parallel()
		got := ResolveReplyToIdentity(context.Background(), stubReplyToResolver{err: errors.New("boom")}, 42, nil)
		assert.True(t, got.IsZero())
	})

	t.Run("nil resolver is zero", func(t *testing.T) {
		t.Parallel()
		assert.True(t, ResolveReplyToIdentity(context.Background(), nil, 42, nil).IsZero())
	})

	t.Run("identity passes through", func(t *testing.T) {
		t.Parallel()
		got := ResolveReplyToIdentity(context.Background(), stubReplyToResolver{identity: ReplyToIdentity{
			Name:    "OGS Am Berg",
			Address: "ogs@schule.example",
		}}, 42, nil)
		assert.Equal(t, "ogs@schule.example", got.Address)
		assert.Equal(t, "OGS Am Berg", got.Name)
	})
}
