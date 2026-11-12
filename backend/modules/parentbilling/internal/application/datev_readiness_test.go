package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The readiness check of the DATEV settings (#4694): it reads the settings
// of the session's Träger only and names the keys, never the values.

type datevSettingsDouble struct {
	byOrganization map[int64]parentbilling.DatevSettings
	err            error
	asked          []int64
}

func (d *datevSettingsDouble) DatevSettings(_ context.Context, organizationID int64) (parentbilling.DatevSettings, error) {
	d.asked = append(d.asked, organizationID)
	return d.byOrganization[organizationID], d.err
}

func datevSettingsOf(consultant, client, length, chart, month, revenue string) parentbilling.DatevSettings {
	return parentbilling.DatevSettings{
		ConsultantNumber: parentbilling.DatevSetting{Key: "consultant", Value: consultant},
		ClientNumber:     parentbilling.DatevSetting{Key: "client", Value: client},
		AccountLength:    parentbilling.DatevSetting{Key: "length", Value: length},
		ChartOfAccounts:  parentbilling.DatevSetting{Key: "chart", Value: chart},
		FiscalYearStart:  parentbilling.DatevSetting{Key: "fiscal", Value: month},
		RevenueAccount:   parentbilling.DatevSetting{Key: "revenue", Value: revenue},
	}
}

func newDatevExportForTest(settings parentbilling.DatevBillingSettings) parentbilling.DatevExport {
	return NewDatevExport(settings,
		func() calendar.Date { return calendar.NewDate(2026, time.November, 9) },
		func() time.Time { return time.Date(2026, time.November, 9, 10, 0, 0, 0, calendar.Berlin) })
}

func TestDatevReadinessNamesMissingAndInvalidSettings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		settings         parentbilling.DatevSettings
		missing, invalid []string
	}{
		{name: "registry defaults", settings: datevSettingsOf("", "", "", "", "1", ""),
			missing: []string{"client", "consultant", "length", "revenue"}, invalid: []string{}},
		{name: "complete", settings: datevSettingsOf("29098", "55003", "4", "04", "1", "4400"),
			missing: []string{}, invalid: []string{}},
		{name: "fiscal year starting in August", settings: datevSettingsOf("29098", "55003", "5", "", "8", "44000"),
			missing: []string{}, invalid: []string{}},
		{name: "revenue account longer than the accounts", settings: datevSettingsOf("29098", "55003", "4", "03", "1", "44000"),
			missing: []string{}, invalid: []string{"revenue"}},
		{name: "values out of range", settings: datevSettingsOf("1000", "100000", "9", "SKR", "13", "0"),
			missing: []string{}, invalid: []string{"chart", "client", "consultant", "fiscal", "length"}},
		{name: "revenue account waits for the length", settings: datevSettingsOf("29098", "55003", "", "", "1", "4400"),
			missing: []string{"length"}, invalid: []string{}},
		{name: "no fiscal year", settings: datevSettingsOf("29098", "55003", "4", "", "", "4400"),
			missing: []string{"fiscal"}, invalid: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			settings := &datevSettingsDouble{byOrganization: map[int64]parentbilling.DatevSettings{7: tc.settings}}
			readiness, err := newDatevExportForTest(settings).DatevReadiness(context.Background(),
				parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 3})
			require.NoError(t, err)
			assert.Equal(t, tc.missing, readiness.MissingSettings)
			assert.Equal(t, tc.invalid, readiness.InvalidSettings)
			assert.Equal(t, len(tc.missing)+len(tc.invalid) == 0, readiness.Ready)
		})
	}
}

// The check reads the settings of the session's Träger, never another
// one's: Träger 8 is complete, Träger 7 is not.
func TestDatevReadinessReadsOnlyTheSessionsTraeger(t *testing.T) {
	t.Parallel()
	settings := &datevSettingsDouble{byOrganization: map[int64]parentbilling.DatevSettings{
		7: datevSettingsOf("", "", "", "", "1", ""),
		8: datevSettingsOf("29098", "55003", "4", "04", "1", "4400"),
	}}
	readiness, err := newDatevExportForTest(settings).DatevReadiness(context.Background(),
		parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 3})
	require.NoError(t, err)
	assert.False(t, readiness.Ready)
	assert.Equal(t, []int64{7}, settings.asked)
}

func TestDatevReadinessPropagatesASettingsError(t *testing.T) {
	t.Parallel()
	failure := errors.New("settings down")
	_, err := newDatevExportForTest(&datevSettingsDouble{err: failure}).DatevReadiness(context.Background(),
		parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 3})
	require.ErrorIs(t, err, failure)
}

func TestDatevFiscalYear(t *testing.T) {
	t.Parallel()
	start, end := datevFiscalYear(calendar.NewDate(2026, time.March, 5), time.August)
	assert.Equal(t, calendar.NewDate(2025, time.August, 1), start)
	assert.Equal(t, calendar.NewDate(2026, time.July, 31), end)
	start, end = datevFiscalYear(calendar.NewDate(2026, time.December, 31), time.January)
	assert.Equal(t, calendar.NewDate(2026, time.January, 1), start)
	assert.Equal(t, calendar.NewDate(2026, time.December, 31), end)
}
