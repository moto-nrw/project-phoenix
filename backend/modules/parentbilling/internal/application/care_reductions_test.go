package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

type evidenceLockStore struct {
	CareReductionStore
	locked bool
	err    error
}

func (s *evidenceLockStore) LockCareBilling(context.Context, int64) error {
	s.locked = true
	return nil
}

func (s *evidenceLockStore) ListContributionRules(context.Context, int64, string) ([]parentbilling.ContributionRule, error) {
	return nil, s.err
}

func TestCareEvidenceLocksBeforeReadingRules(t *testing.T) {
	t.Parallel()
	store := &evidenceLockStore{err: errors.New("stop after reading rules")}
	service := &CareReductions{store: store}
	_, err := service.SaveCareEvidence(context.Background(), parentbilling.HolidayReviewer{OrganizationID: 1, AccountID: 1}, 0, parentbilling.CareEvidenceInput{})
	require.ErrorIs(t, err, store.err)
	assert.True(t, store.locked, "rule validation must run under the billing lock")
}
