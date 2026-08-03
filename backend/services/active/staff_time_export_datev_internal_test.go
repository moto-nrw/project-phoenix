package active

import (
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
)

func TestDatevCategoryValue_ExcludesCurrentMonthOpeningFromPlusHours(t *testing.T) {
	minutes, _, _, ok := datevCategoryValue(configSvc.PayrollCategoryStatus{ID: "plus_stunden"}, MonthExportRow{
		BalanceMinutes:      180,
		OpeningMinutes:      120,
		OpeningCarryMinutes: 120,
	})

	assert.True(t, ok)
	assert.Equal(t, 60, minutes)

	minutes, _, _, ok = datevCategoryValue(configSvc.PayrollCategoryStatus{ID: "plus_stunden"}, MonthExportRow{
		BalanceMinutes:      180,
		OpeningCarryMinutes: 120,
	})
	assert.True(t, ok)
	assert.Equal(t, 180, minutes, "a later month's delta must not subtract the carried opening")

	minutes, _, _, ok = datevCategoryValue(configSvc.PayrollCategoryStatus{ID: "plus_stunden"}, MonthExportRow{
		BalanceMinutes:      60,
		OpeningCarryMinutes: 120,
	})
	assert.True(t, ok)
	assert.Equal(t, 60, minutes)
}

// The DATEV files carry no free text today (personnel numbers, Lohnarten,
// dates), but the encoder must still be ANSI-correct the day a text field is
// added — the LODAS handbook prescribes "ANSI-Code" with CR/LF record ends.
func TestEncodeDatevANSI(t *testing.T) {
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
