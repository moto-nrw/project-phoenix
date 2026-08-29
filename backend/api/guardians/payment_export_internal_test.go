package guardians

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/listexport"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

func exportInt64Ptr(v int64) *int64 { return &v }

// TestBuildPaymentExportRows pins that the exported file keeps the incomplete
// children and names what is missing. Dropping them would make a half-filled
// list look finished, which is exactly how a child ends up never being charged.
func TestBuildPaymentExportRows(t *testing.T) {
	t.Parallel()

	rows := buildPaymentExportRows([]usersSvc.GuardianPaymentRow{
		{
			StudentID:         1,
			StudentName:       "Mia Schneider",
			SchoolClass:       "1a",
			GuardianProfileID: exportInt64Ptr(10),
			AccountHolder:     "Sabine Schneider",
			IBAN:              "DE89370400440532013000",
		},
		{
			StudentID:         2,
			StudentName:       "Ben Koch",
			SchoolClass:       "2b",
			GuardianProfileID: exportInt64Ptr(11),
			AccountHolder:     "Stefan Koch",
		},
		{
			StudentID:   3,
			StudentName: "Lea Wolf",
			SchoolClass: "3c",
		},
	})

	require.Len(t, rows, 3)

	assert.Equal(t, "Mia Schneider", rows[0].Values[listexport.ColumnStudentName])
	assert.Equal(t, "Sabine Schneider", rows[0].Values[listexport.ColumnContactName])
	assert.Equal(t, "DE89370400440532013000", rows[0].Values[listexport.ColumnIBAN])

	assert.Equal(t, "Stefan Koch", rows[1].Values[listexport.ColumnContactName])
	assert.Equal(t, "Fehlt", rows[1].Values[listexport.ColumnIBAN],
		"a known payer without bank details must be visible as a gap")

	assert.Equal(t, "Nicht zugeordnet", rows[2].Values[listexport.ColumnContactName])
	assert.Equal(t, "Fehlt", rows[2].Values[listexport.ColumnIBAN])
	assert.Equal(t, "3c", rows[2].Values[listexport.ColumnStudentClass])
}

// TestPaymentExportSubtitle pins that the document says how complete it is.
func TestPaymentExportSubtitle(t *testing.T) {
	t.Parallel()

	complete := []usersSvc.GuardianPaymentRow{
		{IBAN: "DE89370400440532013000"},
		{IBANMasked: "•••• 3000"},
	}
	assert.Equal(t, "2 Kinder, alle mit Bankverbindung", paymentExportSubtitle(complete))

	partial := append([]usersSvc.GuardianPaymentRow{{}}, complete...)
	assert.Equal(t, "3 Kinder, davon 2 mit Bankverbindung", paymentExportSubtitle(partial))

	assert.Equal(t, "0 Kinder, alle mit Bankverbindung", paymentExportSubtitle(nil))
}
