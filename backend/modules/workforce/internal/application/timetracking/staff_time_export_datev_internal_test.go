package timetracking

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatevCategoryValue_ExcludesCurrentMonthOpeningFromPlusHours(t *testing.T) {
	t.Parallel()

	minutes, _, _, ok := datevCategoryValue(PayrollExportCategory{ID: "plus_stunden"}, MonthExportRow{
		BalanceMinutes: 180,
		OpeningMinutes: 120,
	})

	assert.True(t, ok)
	assert.Equal(t, 60, minutes)

	minutes, _, _, ok = datevCategoryValue(PayrollExportCategory{ID: "plus_stunden"}, MonthExportRow{
		BalanceMinutes: 180,
	})
	assert.True(t, ok)
	assert.Equal(t, 180, minutes, "a later month's delta must not subtract the carried opening")

	minutes, _, _, ok = datevCategoryValue(PayrollExportCategory{ID: "plus_stunden"}, MonthExportRow{
		BalanceMinutes: 60,
	})
	assert.True(t, ok)
	assert.Equal(t, 60, minutes)
}

type datevOverviewStub struct{ rows []MonthExportRow }

func (s datevOverviewStub) GetDashboardSummary(context.Context, string) (*DashboardSummary, error) {
	return nil, nil
}

func (s datevOverviewStub) GetTimeTrackingOverview(context.Context, OverviewFilters) (*TimeTrackingOverview, error) {
	return nil, nil
}

func (s datevOverviewStub) GetMonthExportRows(context.Context, int, int) ([]MonthExportRow, error) {
	return s.rows, nil
}

type payrollConfigurationStub struct{ configuration PayrollExportConfiguration }

func (s payrollConfigurationStub) GetPayrollConfiguration(context.Context) (*PayrollExportConfiguration, error) {
	return &s.configuration, nil
}

type payrollRuleSetStub struct{}

func (payrollRuleSetStub) CurrentPayrollRuleSet(context.Context) (*workforce.PayrollRuleSet, error) {
	return nil, nil
}

func TestDatevPreflight_UsesTheRuleSetOfTheRequestedRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		rows          []MonthExportRow
		configuration PayrollExportConfiguration
		want          error
		wantDetail    string
	}{
		{
			name: "numbered frozen wage type allows a historical export",
			rows: []MonthExportRow{{
				PayrollRules: &workforce.PayrollRuleSet{
					WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Historisch", Number: "310", Unit: workforce.WageTypeUnitHours}},
				},
			}},
		},
		{
			name: "unnumbered frozen wage type does not use a current number",
			rows: []MonthExportRow{{
				PayrollRules: &workforce.PayrollRuleSet{
					WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Historisch", Unit: workforce.WageTypeUnitHours}},
				},
			}},
			want: ErrPayrollConfigIncomplete,
		},
		{
			name: "an own wage type may not reuse a fixed category number",
			rows: []MonthExportRow{{
				PayrollRules: &workforce.PayrollRuleSet{
					WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitHours}},
				},
			}},
			configuration: PayrollExportConfiguration{
				Categories:           []PayrollExportCategory{{Label: "Regelarbeit", Number: "310"}},
				ConfiguredCategories: 1,
			},
			want:       ErrPayrollConfigIncomplete,
			wantDetail: "310",
		},
		{
			name: "two own wage types may not reuse a number",
			rows: []MonthExportRow{{
				PayrollRules: &workforce.PayrollRuleSet{WageTypes: []workforce.PayrollWageType{
					{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitHours},
					{ID: 8, Name: "Feiertagszuschlag", Number: "310", Unit: workforce.WageTypeUnitHours},
				}},
			}},
			want:       ErrPayrollConfigIncomplete,
			wantDetail: "310",
		},
		{
			name: "the same frozen wage type on several rows remains valid",
			rows: []MonthExportRow{
				{PayrollRules: &workforce.PayrollRuleSet{WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitHours}}}},
				{PayrollRules: &workforce.PayrollRuleSet{WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitHours}}}},
			},
		},
		{
			name: "a frozen and current unit may not reuse a wage type number",
			rows: []MonthExportRow{
				{PayrollRules: &workforce.PayrollRuleSet{WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitHours}}}},
				{PayrollRules: &workforce.PayrollRuleSet{WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Nachtzuschlag", Number: "310", Unit: workforce.WageTypeUnitDays}}}},
			},
			want:       ErrPayrollConfigIncomplete,
			wantDetail: "310",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := &staffTimeExportService{
				overview:      datevOverviewStub{rows: test.rows},
				payrollStatus: payrollConfigurationStub{configuration: test.configuration},
				payrollRules:  payrollRuleSetStub{},
			}

			_, _, _, err := svc.buildDatev(context.Background(), TimeExportRequest{
				Year: 2026, Month: 6, Format: ExportFormatDatevLug,
			})
			if test.want != nil {
				require.ErrorIs(t, err, test.want)
				assert.ErrorContains(t, err, test.wantDetail)
				return
			}
			require.NoError(t, err)
		})
	}
}

// The DATEV files carry no free text today (personnel numbers, Lohnarten,
// dates), but the encoder must still be ANSI-correct the day a text field is
// added — the LODAS handbook prescribes "ANSI-Code" with CR/LF record ends.
func TestEncodeDatevANSI(t *testing.T) {
	t.Parallel()

	out := encodeDatevANSI("Bäcker-Größe;1\n")

	// Windows-1252: ä = 0xE4, ö = 0xF6 — a UTF-8 pass-through would leak
	// two-byte sequences and the payroll import would garble the umlauts.
	assert.Equal(t, []byte{
		'B', 0xE4, 'c', 'k', 'e', 'r', '-', 'G', 'r', 0xF6, 0xDF, 'e', ';', '1', '\r', '\n',
	}, out)

	// Characters outside CP1252 are substitution-encoded, never dropped and
	// never a hard failure for the whole file.
	sub := encodeDatevANSI("€ und ☃")
	assert.Equal(t, byte(0x80), sub[0], "the euro sign IS part of CP1252")
	assert.NotContains(t, string(sub), "☃")
}
