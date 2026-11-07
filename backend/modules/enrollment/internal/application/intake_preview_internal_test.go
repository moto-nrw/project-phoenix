package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

type previewErrorCatalog struct {
	IntakeCatalog
	err error
}

func (c previewErrorCatalog) Phase(context.Context, int64) (*enrollment.Phase, error) {
	return nil, c.err
}

func TestLoadPhasePreviewCareOfferingsPhaseErrors(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"missing phase", fmt.Errorf("phase not found: %w", sql.ErrNoRows), enrollment.ErrPhaseNotFound},
		{"database failure", fmt.Errorf("load phase: %w", dbErr), dbErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewIntake(IntakeDependencies{
				Settings: intakeSettingsFake{},
				Catalog:  previewErrorCatalog{err: tc.err},
				Runtime:  Runtime{NotFound: NewErrorMatcher(sql.ErrNoRows)},
			})
			data, err := svc.LoadPhasePreviewCareOfferings(context.Background(), 42)
			require.Nil(t, data)
			require.ErrorIs(t, err, tc.want)
			if errors.Is(tc.want, dbErr) {
				require.NotErrorIs(t, err, enrollment.ErrPhaseNotFound)
			}
		})
	}
}
