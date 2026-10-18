package legacy

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareNamesUsesGermanCollation(t *testing.T) {
	t.Parallel()

	names := []string{"Zebra", "Äpfel", "apfel"}
	slices.SortStableFunc(names, CompareNames)

	assert.Equal(t, []string{"apfel", "Äpfel", "Zebra"}, names,
		"umlauts sort with their base letter, ahead of Z and after the plain letter (DIN 5007-1)")
}
