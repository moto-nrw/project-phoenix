package fileformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapStudentRow_GuardianNumberingWithGaps(t *testing.T) {
	t.Parallel()

	headers := []string{"Vorname", "Nachname", "Klasse", "Erz1.Email", "Erz3.Mobil", "Erz3.Rolle", "Erz3.Abholhinweis", "Erz3.Notfallpriorität"}
	mapping := make(map[string]int, len(headers))
	for i, h := range headers {
		mapping[normalizeHeaderKey(h)] = i
	}
	mapper := NewColumnMapper(mapping, []string{"Max", "Muster", "1A", "m@example.test", "0171-1", "Nur Abholung", "nur dienstags", "2"})

	row, err := MapStudentRow(mapper)
	require.NoError(t, err)
	require.Len(t, row.Guardians, 2, "Erz3 must be read although Erz2 columns are missing")
	assert.Equal(t, "Nur Abholung", row.Guardians[1].GuardianRole)
	assert.Equal(t, "nur dienstags", row.Guardians[1].PickupNotes)
	assert.Equal(t, 2, row.Guardians[1].EmergencyPriority)
}
