package textnorm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

func TestReadCSVTableReadsHeadersRowsAndLines(t *testing.T) {
	t.Parallel()
	table, err := textnorm.ReadCSVTable([]byte("\x00Name \x00;\x00 Ort\x00\r\n\r\n\x00Eva \"Evi\"\x00;\x00Köln\x00\r\n;\r\nJan;Bonn\r\n"), "", 10)
	require.NoError(t, err)
	assert.Equal(t, "utf-8", table.Encoding)
	assert.Equal(t, ";", table.Delimiter)
	assert.Equal(t, []string{"Name", "Ort"}, table.Headers)
	assert.Equal(t, [][]string{{"Eva \"Evi\"", "Köln"}, {"Jan", "Bonn"}}, table.Rows)
	assert.Equal(t, []int{3, 5}, table.Lines, "blank records keep their line out of the rows")
}

func TestReadCSVTableRefusesWhatIsNoTextAndTooManyRows(t *testing.T) {
	t.Parallel()
	_, err := textnorm.ReadCSVTable([]byte("PK\x03\x04\x00\x00binary"), "", 10)
	require.ErrorIs(t, err, textnorm.ErrCSVInvalid)
	_, err = textnorm.ReadCSVTable([]byte("a;b\nc;\x00\n"), "", 10)
	require.ErrorIs(t, err, textnorm.ErrCSVInvalid, "a NUL that is no quote")
	_, err = textnorm.ReadCSVTable([]byte("a;b\n1;2\n"), "|", 10)
	require.ErrorIs(t, err, textnorm.ErrCSVInvalid, "an unknown delimiter")
	_, err = textnorm.ReadCSVTable([]byte("a;b\n1;2\n3;4\n"), "", 1)
	require.ErrorIs(t, err, textnorm.ErrCSVTooManyRows)
}

func TestHeaderKeyAndRepeatedCSVHeader(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "glaeubigeridnr", textnorm.HeaderKey("Gläubiger-ID Nr."))
	assert.True(t, textnorm.RepeatedCSVHeader([]string{"Name", "", "Name"}))
	assert.False(t, textnorm.RepeatedCSVHeader([]string{"Name", "", ""}))
}
