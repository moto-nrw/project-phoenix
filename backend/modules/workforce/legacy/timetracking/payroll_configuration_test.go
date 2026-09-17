package timetracking_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payrollStatusSource func(context.Context) (*config.PayrollStatus, error)

func (f payrollStatusSource) GetPayrollStatus(ctx context.Context) (*config.PayrollStatus, error) {
	return f(ctx)
}

func TestPayrollExportSettings_ProjectsResolvedConfiguration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status := &config.PayrollStatus{
		Beraternummer: "1234567", Mandantennummer: "54321",
		LodasHeaderComplete: true, ConfiguredCategories: 1,
		Categories: []config.PayrollCategoryStatus{{
			ID: "krank", Label: "Krankheit", Number: "200", Unit: "tage", UnitRequired: true,
		}},
	}
	adapter := services.PayrollExportSettings{Source: payrollStatusSource(func(received context.Context) (*config.PayrollStatus, error) {
		require.Same(t, ctx, received)
		return status, nil
	})}
	got, err := adapter.GetPayrollConfiguration(ctx)
	require.NoError(t, err)
	assert.Equal(t, &timetracking.PayrollExportConfiguration{
		Beraternummer: "1234567", Mandantennummer: "54321",
		LodasHeaderComplete: true, ConfiguredCategories: 1,
		Categories: []timetracking.PayrollExportCategory{{
			ID: "krank", Label: "Krankheit", Number: "200", Unit: "tage", UnitRequired: true,
		}},
	}, got)
	got.Categories[0].Number = "changed"
	assert.Equal(t, "200", status.Categories[0].Number)
}

func TestPayrollExportSettings_PropagatesResolutionFailure(t *testing.T) {
	t.Parallel()
	want := errors.New("settings unavailable")
	adapter := services.PayrollExportSettings{Source: payrollStatusSource(func(context.Context) (*config.PayrollStatus, error) {
		return nil, want
	})}
	got, err := adapter.GetPayrollConfiguration(context.Background())
	require.ErrorIs(t, err, want)
	assert.Nil(t, got)
}

func TestPayrollExportSettings_RejectsMissingConfiguration(t *testing.T) {
	t.Parallel()
	adapter := services.PayrollExportSettings{Source: payrollStatusSource(func(context.Context) (*config.PayrollStatus, error) {
		return nil, nil
	})}
	got, err := adapter.GetPayrollConfiguration(context.Background())
	require.Error(t, err)
	assert.Nil(t, got)
}
