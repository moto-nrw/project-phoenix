package api

import (
	"context"
	"errors"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The payroll status is the Settings Platform's record, which the root names
// only through the Settings composition. The fixtures below start from the
// status the payroll status service resolves for an unconfigured school and
// serve it through a source that is generic over the status type.

// fixedPayrollStatus serves one status (or failure) and records the context
// it was asked with.
type fixedPayrollStatus[S any] struct {
	status   S
	err      error
	received *context.Context
}

func (f fixedPayrollStatus[S]) GetPayrollStatus(ctx context.Context) (S, error) {
	if f.received != nil {
		*f.received = ctx
	}
	return f.status, f.err
}

func newFixedPayrollStatus[S any](status S, err error, received *context.Context) fixedPayrollStatus[S] {
	return fixedPayrollStatus[S]{status: status, err: err, received: received}
}

// noPayrollStatus is the absent status of the type of status: a nil record.
func noPayrollStatus[S any](S) S {
	var none S
	return none
}

// unconfiguredPayrollSettings answers every payroll setting empty.
type unconfiguredPayrollSettings struct{}

func (unconfiguredPayrollSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

// unconfiguredPayrollStatus resolves the status of a school without payroll
// configuration.
func unconfiguredPayrollStatus(t *testing.T) settingsCompose.PayrollStatusGetter {
	t.Helper()
	return settingsCompose.NewPayrollStatusService(unconfiguredPayrollSettings{}, func(context.Context) (int, int, error) {
		return 0, 0, nil
	})
}

func TestPayrollExportSettings_ProjectsResolvedConfiguration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status, err := unconfiguredPayrollStatus(t).GetPayrollStatus(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, status.Categories)
	status.Beraternummer, status.Mandantennummer = "1234567", "54321"
	status.LodasHeaderComplete, status.ConfiguredCategories = true, 1
	category := status.Categories[0]
	category.ID, category.Label, category.Number, category.Unit, category.UnitRequired = "krank", "Krankheit", "200", "tage", true
	status.Categories = append(status.Categories[:0], category)

	var received context.Context
	adapter := owners.PayrollExportSettings{Source: newFixedPayrollStatus(status, nil, &received)}
	got, err := adapter.GetPayrollConfiguration(ctx)
	require.NoError(t, err)
	require.Same(t, ctx, received)
	assert.Equal(t, &workforceCompose.PayrollExportConfiguration{
		Beraternummer: "1234567", Mandantennummer: "54321",
		LodasHeaderComplete: true, ConfiguredCategories: 1,
		Categories: []workforceCompose.PayrollExportCategory{{
			ID: "krank", Label: "Krankheit", Number: "200", Unit: "tage", UnitRequired: true,
		}},
	}, got)
	got.Categories[0].Number = "changed"
	assert.Equal(t, "200", status.Categories[0].Number)
}

func TestPayrollExportSettings_PropagatesResolutionFailure(t *testing.T) {
	t.Parallel()
	status, err := unconfiguredPayrollStatus(t).GetPayrollStatus(context.Background())
	require.NoError(t, err)
	want := errors.New("settings unavailable")
	adapter := owners.PayrollExportSettings{Source: newFixedPayrollStatus(noPayrollStatus(status), want, nil)}
	got, err := adapter.GetPayrollConfiguration(context.Background())
	require.ErrorIs(t, err, want)
	assert.Nil(t, got)
}

func TestPayrollExportSettings_RejectsMissingConfiguration(t *testing.T) {
	t.Parallel()
	status, err := unconfiguredPayrollStatus(t).GetPayrollStatus(context.Background())
	require.NoError(t, err)
	adapter := owners.PayrollExportSettings{Source: newFixedPayrollStatus(noPayrollStatus(status), nil, nil)}
	got, err := adapter.GetPayrollConfiguration(context.Background())
	require.Error(t, err)
	assert.Nil(t, got)
}
