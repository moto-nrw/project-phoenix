package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const guardianPhoneSearchDigitsVersion = "20261114065713"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianPhoneSearchDigitsVersion,
		Description: "Telefonnummern der Kontaktpersonen: gespeicherter Suchschlüssel für die Schulsuche (#6954)",
		DependsOn: []string{
			GuardianPhoneNumbersVersion, // users.guardian_phone_numbers, the table it alters (1.7.6)
		},
	})
	Migrations.MustRegister(guardianPhoneSearchDigitsUp, guardianPhoneSearchDigitsDown)
}

// guardianPhoneSearchDigitsUpSQL stores the search key of every contact phone
// number (ADR 0231). The school search compares a typed number with this key;
// computing it per row at read time cost three regular expressions per
// comparison and was the most expensive statement of a search.
//
//   - public.f_phone_search_key is the database twin of
//     schoolsearchview.PhoneNumberKey: the text compacted to digits, plus and
//     brackets without the bracketed trunk zero, then its digits without the
//     international 00, the country code 49 of an international number and the
//     German trunk zero. A 49 without + or 00 stays. The projection's
//     TestPhoneSearchKeySQLMatchesGo holds both to the same cases.
//   - search_digits is a generated column: the database fills it on every
//     insert and update, so no writer can leave it stale. It holds nothing the
//     number itself does not hold and goes with its row, also into the data
//     return of a school (return_register.json).
//   - The function is IMMUTABLE because the stored keys depend on it. A change
//     of its reading must rewrite the column in the same migration
//     (ALTER TABLE ... ALTER COLUMN search_digits SET EXPRESSION).
//   - No index: the search reads the numbers of one school through the tenant
//     index and matches partial numbers as a part of the key. Under row level
//     security the planner uses no index for LIKE or strpos, because neither
//     is leakproof.
//
// Adding the column rewrites the table once under its lock; it holds a few
// numbers per family.
const guardianPhoneSearchDigitsUpSQL = `
CREATE FUNCTION public.f_phone_search_key(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
BEGIN ATOMIC
  SELECT CASE WHEN NOT k.intl OR k.d2 LIKE '0%' THEN ltrim(k.d2, '0') ELSE k.d2 END
  FROM (SELECT j.intl, CASE WHEN j.intl AND j.d1 LIKE '49%' THEN substr(j.d1, 3) ELSE j.d1 END AS d2
    FROM (SELECT i.intl, regexp_replace(i.digits, '^00', '') AS d1
      FROM (SELECT ltrim(c.compact, '()') LIKE '+%' OR regexp_replace(c.compact, '[^0-9]', '', 'g') LIKE '00%' AS intl,
          regexp_replace(c.compact, '[^0-9]', '', 'g') AS digits
        FROM (SELECT replace(regexp_replace($1, '[^0-9+()]', '', 'g'), '(0)', '') AS compact) AS c) AS i) AS j) AS k;
END;
COMMENT ON FUNCTION public.f_phone_search_key(text) IS
	'Search key of a phone number as written (ADR 0231); twin of schoolsearchview.PhoneNumberKey.';
ALTER TABLE users.guardian_phone_numbers
	ADD COLUMN search_digits TEXT GENERATED ALWAYS AS (public.f_phone_search_key(phone_number)) STORED;
COMMENT ON COLUMN users.guardian_phone_numbers.search_digits IS
	'Generated: public.f_phone_search_key(phone_number), read by the school search (ADR 0231).';
`

const guardianPhoneSearchDigitsDownSQL = `
ALTER TABLE users.guardian_phone_numbers DROP COLUMN IF EXISTS search_digits;
DROP FUNCTION IF EXISTS public.f_phone_search_key(text);
`

func guardianPhoneSearchDigitsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, guardianPhoneSearchDigitsUpSQL); err != nil {
		return fmt.Errorf("add guardian phone search key: %w", err)
	}
	return nil
}

func guardianPhoneSearchDigitsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, guardianPhoneSearchDigitsDownSQL); err != nil {
		return fmt.Errorf("drop guardian phone search key: %w", err)
	}
	return nil
}
