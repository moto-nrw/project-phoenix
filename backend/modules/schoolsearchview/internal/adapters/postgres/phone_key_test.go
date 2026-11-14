package postgres

import (
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }

// phoneNumbersAsWritten are the ways people write a number: every reading of
// a number must give them the same key.
var phoneNumbersAsWritten = []string{
	"+49 170 1234567", "+491701234567", "0049 170 1234567", "0049-170-1234567", "+49 (0)170 1234567",
	"0170 1234567", "0170/1234567", "0170-123 45 67", "(0170) 123.45.67", "  0221 98765  ",
	"+43 1 2345678", "0043 1 2345678", "1234567", "+4930 1234", "00 49 30 1234",
	"0170\u00a01234567", "0170\u202f123\u00a045\u00a067", "0170\t1234567", "0170\u20131234567",
	"0170\u20111234567", "0170\u22121234567", "\u00a0+49\u00a0170\u00a01234567",
	"+43 (0)1 2345678", "0043 (0) 1 2345678", "49 221 5550001", "4930 1234",
	"+43 (\u00a00)1 2345678", "+43 (\u20020\t)1 2345678",
	// Stored numbers keep a leading 49 typed without + or 00: Emden's 04921
	// without its 0 stays 4921.
	"4921 1234567", "04921 1234567", "4921/1234567", "(+49) 170 1234567",
	"0221 5550001-12", "Tel. +49 221 5550001", "0170", "",
}

// TestPhoneSearchKeySQLMatchesGo holds the SQL reading of a stored number to
// the Go reading of a typed one: both must yield the same key for every way
// of writing a number.
func TestPhoneSearchKeySQLMatchesGo(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, number := range phoneNumbersAsWritten {
		var key string
		require.NoError(t, db.NewRaw(`SELECT `+phoneKeyOfStoredNumber+` FROM (SELECT CAST(? AS text) AS phone_number) AS phone`, number).
			Scan(testpkg.Ctx(t), &key), number)
		require.Equal(t, schoolsearchview.PhoneNumberKey(number), key, number)
	}
}

// TestStoredPhoneSearchKeyMatchesGo holds the key the database stores with a
// number (the generated column the phone search reads) to the Go reading, on
// insert and after the number changes.
func TestStoredPhoneSearchKeyMatchesGo(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Kai", "Schlüssel", "kai.schluessel")
	for _, number := range phoneNumbersAsWritten {
		_, err := db.ExecContext(ctx, `INSERT INTO users.guardian_phone_numbers (tenant_id, guardian_profile_id, phone_number, phone_type, is_primary)
			VALUES (?, ?, ?, 'mobile', false)`, testpkg.Tenant(t), guardian.ID, number)
		require.NoError(t, err, number)
	}
	stored := func() map[string]string {
		var rows []struct {
			PhoneNumber  string `bun:"phone_number"`
			SearchDigits string `bun:"search_digits"`
		}
		require.NoError(t, db.NewRaw(`SELECT phone.phone_number, phone.search_digits FROM users.guardian_phone_numbers AS phone
			WHERE phone.tenant_id = ? AND phone.guardian_profile_id = ?`, testpkg.Tenant(t), guardian.ID).Scan(ctx, &rows))
		keys := make(map[string]string, len(rows))
		for _, row := range rows {
			keys[row.PhoneNumber] = row.SearchDigits
		}
		return keys
	}
	keys := stored()
	require.Len(t, keys, len(phoneNumbersAsWritten))
	for _, number := range phoneNumbersAsWritten {
		require.Equal(t, schoolsearchview.PhoneNumberKey(number), keys[number], number)
	}

	_, err := db.ExecContext(ctx, `UPDATE users.guardian_phone_numbers SET phone_number = '0049 (0)30 7654321'
		WHERE tenant_id = ? AND guardian_profile_id = ? AND phone_number = '1234567'`, testpkg.Tenant(t), guardian.ID)
	require.NoError(t, err)
	require.Equal(t, "307654321", stored()["0049 (0)30 7654321"], "a changed number gets its new key")
}

// TestPhoneSQLSpellsOutTheGoBounds keeps the numbers the SQL spells out equal
// to the Go constants they stand for.
func TestPhoneSQLSpellsOutTheGoBounds(t *testing.T) {
	t.Parallel()
	require.NotContains(t, phoneKeyOfStoredNumber, "length(j.d1) >= ", "a stored number never loses a 49 typed without + or 00")
	require.Contains(t, phoneExtensionMatch, "length(candidate.key) >= "+strconv.Itoa(schoolsearchview.MinStoredPhoneDigits))
	require.Contains(t, phoneExtensionMatch, "BETWEEN 1 AND "+strconv.Itoa(schoolsearchview.MaxExtensionDigits))
}
