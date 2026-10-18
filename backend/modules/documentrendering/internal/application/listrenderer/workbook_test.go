package listexport

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestRenderWorkbookPreservesTypesAndStyles(t *testing.T) {
	t.Parallel()
	data, err := RenderWorkbook("Zeiterfassung", []string{"Name", "Stunden"}, [][]any{{"=untrusted", 7.5}}, []int{2})
	require.NoError(t, err)
	book, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer func() { require.NoError(t, book.Close()) }()
	require.Equal(t, []string{"Zeiterfassung"}, book.GetSheetList())
	value, err := book.GetCellValue("Zeiterfassung", "A2")
	require.NoError(t, err)
	require.Equal(t, "=untrusted", value)
	formula, err := book.GetCellFormula("Zeiterfassung", "A2")
	require.NoError(t, err)
	require.Empty(t, formula)
	cellType, err := book.GetCellType("Zeiterfassung", "B2")
	require.NoError(t, err)
	// Excelize writes numeric floats with the default (unset) cell type.
	require.Equal(t, excelize.CellTypeUnset, cellType)
	rawValue, err := book.GetCellValue("Zeiterfassung", "B2", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Equal(t, "7.5", rawValue)
	value, err = book.GetCellValue("Zeiterfassung", "B2")
	require.NoError(t, err)
	require.Equal(t, "7.50", value)
	styleID, err := book.GetCellStyle("Zeiterfassung", "A1")
	require.NoError(t, err)
	style, err := book.GetStyle(styleID)
	require.NoError(t, err)
	require.NotNil(t, style.Font)
	require.True(t, style.Font.Bold)
	width, err := book.GetColWidth("Zeiterfassung", "A")
	require.NoError(t, err)
	require.Equal(t, float64(16), width)
}

func TestRenderWorkbookRejectsInvalidSheetName(t *testing.T) {
	t.Parallel()
	data, err := RenderWorkbook("invalid/name", nil, nil, nil)
	require.ErrorContains(t, err, "failed to create sheet")
	require.Nil(t, data)
}
