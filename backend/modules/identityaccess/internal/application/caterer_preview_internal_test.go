package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type previewCatererStore struct {
	ports.CatererStore
	invitation   domain.CatererInvitation
	accountReads int
}

func (s *previewCatererStore) FindCatererInvitationByTokenHash(context.Context, string) (domain.CatererInvitation, bool, domain.OperationStats, error) {
	return s.invitation, true, domain.OperationStats{}, nil
}

func (s *previewCatererStore) FindLoginAccountByEmail(context.Context, string) (domain.LoginAccount, bool, domain.OperationStats, error) {
	s.accountReads++
	return domain.LoginAccount{}, false, domain.OperationStats{}, nil
}

func TestCatererPreviewPropagatesSchoolLookupFailure(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	lookupErr := errors.New("school directory unavailable")
	f.schools.findErr = lookupErr
	now := time.Now()
	store := &previewCatererStore{invitation: domain.CatererInvitation{TenantID: 10, Email: "kitchen@example.com", ExpiresAt: now.Add(time.Hour)}}
	caterers := &CatererAccess{store: store, schools: f.schools, runtime: f.runtime, now: func() time.Time { return now }}
	err := f.runtime.WithTenantTx(context.Background(), 10, func(ctx context.Context) error {
		preview, err := caterers.preview(ctx, "token")
		assert.Zero(t, preview)
		return err
	})
	require.ErrorIs(t, err, lookupErr)
	assert.Zero(t, store.accountReads, "a school lookup failure stops the preview")
}
