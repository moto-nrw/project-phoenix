package config

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Payroll status (#1417): the same completeness verdict the /payroll page
// shows and the later DATEV writers check before producing a file. Settings
// come from a narrow fake; staff counts use the injected port.

type personnelCounts struct{ total, missing int }

func payrollStatusFixture(t *testing.T, values map[string]string) (PayrollStatusGetter, *personnelCounts, context.Context) {
	t.Helper()
	settings := &fakeSettingsService{opts: fakeSettingsServiceOpts{}}
	settings.resolveString = func(_ context.Context, key string) (string, error) { return values[key], nil }
	counts := &personnelCounts{}
	counter := func(context.Context) (int, int, error) { return counts.total, counts.missing, nil }
	return NewPayrollStatusService(settings, counter), counts, context.Background()
}

func TestPayrollStatus_EmptyConfigurationIsReportedNotInvented(t *testing.T) {
	t.Parallel()

	svc, _, ctx := payrollStatusFixture(t, map[string]string{})

	status, err := svc.GetPayrollStatus(ctx)
	require.NoError(t, err)

	assert.Equal(t, 7, status.TotalCategories)
	assert.Equal(t, 0, status.ConfiguredCategories)
	assert.False(t, status.LodasHeaderComplete)
	assert.Equal(t, "", status.Beraternummer)
	require.Len(t, status.Categories, 7)
	for _, cat := range status.Categories {
		assert.Equal(t, "", cat.Number, "no category may carry an invented default: %s", cat.ID)
	}

	// The three day-value categories carry the unit requirement.
	unitRequired := map[string]bool{}
	for _, cat := range status.Categories {
		unitRequired[cat.ID] = cat.UnitRequired
	}
	assert.True(t, unitRequired["krank"])
	assert.True(t, unitRequired["urlaub"])
	assert.True(t, unitRequired["fortbildung"])
	assert.False(t, unitRequired["regelarbeit"])
	assert.False(t, unitRequired["auszahlung"])
}

func TestPayrollStatus_CompletenessCounting(t *testing.T) {
	t.Parallel()

	svc, _, ctx := payrollStatusFixture(t, map[string]string{
		configModel.KeyPayrollLohnartRegelarbeit:   "9001",
		configModel.KeyPayrollLohnartKrank:         "9002", // no unit yet → not complete
		configModel.KeyPayrollLohnartUrlaub:        "9003",
		configModel.KeyPayrollEinheitUrlaub:        configModel.PayrollUnitDays,
		configModel.KeyPayrollDatevBeraternummer:   "9999999",
		configModel.KeyPayrollDatevMandantennummer: "99999",
	})

	status, err := svc.GetPayrollStatus(ctx)
	require.NoError(t, err)

	// regelarbeit (no unit needed) + urlaub (number AND unit); krank lacks
	// its unit and must not count as configured.
	assert.Equal(t, 2, status.ConfiguredCategories)
	assert.True(t, status.LodasHeaderComplete)
}

func TestPayrollStatus_CountsStaffWithoutPersonnelNumber(t *testing.T) {
	t.Parallel()

	svc, counts, ctx := payrollStatusFixture(t, map[string]string{})

	before, err := svc.GetPayrollStatus(ctx)
	require.NoError(t, err)

	counts.total += 2
	counts.missing++

	after, err := svc.GetPayrollStatus(ctx)
	require.NoError(t, err)

	assert.Equal(t, before.StaffTotal+2, after.StaffTotal)
	assert.Equal(t, before.StaffWithoutPersonnelNumber+1, after.StaffWithoutPersonnelNumber,
		"exactly one of the two new staff lacks a personnel number")
}
