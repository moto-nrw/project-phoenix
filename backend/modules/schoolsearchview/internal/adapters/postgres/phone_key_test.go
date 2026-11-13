package postgres

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }

// TestPhoneSearchKeySQLMatchesGo holds the SQL reading of a stored number to
// the Go reading of a typed one: both must yield the same key for every way
// of writing a number.
func TestPhoneSearchKeySQLMatchesGo(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, number := range []string{
		"+49 170 1234567", "+491701234567", "0049 170 1234567", "0049-170-1234567", "+49 (0)170 1234567",
		"0170 1234567", "0170/1234567", "0170-123 45 67", "(0170) 123.45.67", "  0221 98765  ",
		"+43 1 2345678", "0043 1 2345678", "1234567", "+4930 1234", "00 49 30 1234",
	} {
		var key string
		require.NoError(t, db.NewRaw(`SELECT `+phoneKeyOfStoredNumber+` FROM (SELECT CAST(? AS text) AS phone_number) AS phone`, number).
			Scan(testpkg.Ctx(t), &key), number)
		require.Equal(t, schoolsearchview.PhoneSearchKey(number), key, number)
	}
}
