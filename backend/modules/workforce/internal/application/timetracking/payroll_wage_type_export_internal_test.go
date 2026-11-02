package timetracking

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A closed month retains the wage type that existed when it was closed. If the
// same type later changes unit, the cross-month export needs two columns so
// neither value is rendered in the other's unit.
func TestMonthExportWageTypeColumns_SeparateChangedUnits(t *testing.T) {
	t.Parallel()

	rows := []MonthExportRow{
		{
			PayrollRules: &workforce.PayrollRuleSet{
				WageTypes:            []workforce.PayrollWageType{{ID: 7, Name: "Bereitschaft", Unit: workforce.WageTypeUnitHours}},
				AbsenceTypeWageTypes: map[int64]int64{1: 7},
			},
			AbsenceTypeCredits: map[int64]AbsenceTypeCredit{1: {Minutes: 60}},
		},
		{
			PayrollRules: &workforce.PayrollRuleSet{
				WageTypes: []workforce.PayrollWageType{{ID: 7, Name: "Bereitschaft", Unit: workforce.WageTypeUnitAmount}},
				SurchargeRules: []workforce.PayrollSurchargeRule{{
					ID: 1, RateType: SurchargeRateAmount, RateHundredths: 250, WageTypeID: 7,
				}},
			},
			SurchargeMinutes: map[int64]int{1: 60},
		},
	}

	csv, err := writeMonthCSV(rows, ExportTimeDecimal)
	assert.NoError(t, err)
	lines := strings.Split(strings.TrimPrefix(strings.TrimSpace(string(csv)), "\ufeff"), "\n")
	require.Len(t, lines, 3)

	assert.Equal(t, []string{"Bereitschaft (Stunden)", "Bereitschaft (Euro)"}, tailCells(lines[0], 2))
	assert.Equal(t, []string{"1,00", ""}, tailCells(lines[1], 2))
	assert.Equal(t, []string{"", "2,50"}, tailCells(lines[2], 2))

	var xlsxHeaders []string
	var xlsxRows [][]any
	_, err = writeMonthXLSX(rows, ExportTimeDecimal, func(_ string, headers []string, values [][]any, _ []int) ([]byte, error) {
		xlsxHeaders, xlsxRows = headers, values
		return nil, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"Bereitschaft (Stunden)", "Bereitschaft (Euro)"}, xlsxHeaders[len(xlsxHeaders)-2:])
	assert.Equal(t, []any{float64(1), ""}, xlsxRows[0][len(xlsxRows[0])-2:])
	assert.Equal(t, []any{"", float64(2.5)}, xlsxRows[1][len(xlsxRows[1])-2:])
}

func tailCells(line string, count int) []string {
	cells := strings.Split(line, ";")
	return cells[len(cells)-count:]
}
