package timetracking

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYearQuery_DefaultsToBerlinCalendarYear(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/staff/42/vacation/opening", nil)

	year, err := parseYearQuery(req, "2026-08-24")

	require.NoError(t, err)
	assert.Equal(t, 2026, year)
}
