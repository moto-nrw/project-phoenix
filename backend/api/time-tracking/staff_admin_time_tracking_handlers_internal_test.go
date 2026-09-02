package timetracking

import (
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYearQuery_DefaultsToBerlinCalendarYear(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/staff/42/vacation/opening", nil)

	year, err := parseYearQuery(req, timezone.NewDate(2026, 8, 24))

	require.NoError(t, err)
	assert.Equal(t, timezone.NewDate(2026, 8, 24).Year(), year)
}
