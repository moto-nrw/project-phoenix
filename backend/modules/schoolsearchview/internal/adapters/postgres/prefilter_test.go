package postgres

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestPrefiltersKeepEveryHit compares the prefilters with the match
// conditions they stand in front of, on the same rows: a row that matches a
// condition must pass its prefilter, whatever the letters. The names cover
// what folds or lowers in a special way: accents, ß, ligatures, the dotted İ
// and the Greek final sigma, which lowers by its place in the word.
func TestPrefiltersKeepEveryHit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	group := testpkg.CreateTestEducationGroup(t, db, "ΛΥΚΟΣ Füchse")
	people := [][2]string{
		{"Émile", "Ødegård"}, {"ΟΔΥΣΣΕΑΣ", "ΠΑΠΑΔΟΠΟΥΛΟΣ"}, {"İpek", "Yıldız"}, {"Jörg", "Straßer"},
		{"Anna-Lena", "von der Heide"}, {"Æsa", "Œuvre"}, {"Li", "Na"},
	}
	for i, name := range people {
		child := testpkg.CreateTestStudent(t, db, name[0], name[1], "2ß ΤΑΞΗΣ")
		_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET group_id = ? WHERE student_profile_id = ?`, group.ID, child.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE users.student_profiles SET extra_info = ? WHERE id = ?`, "Holt die Oma ab, ΟΔΟΣ Straße "+name[1], child.ID)
		require.NoError(t, err)
		testpkg.CreateTestGuardianProfileNamed(t, db, name[1], name[0], "eltern"+string(rune('a'+i)))
		testpkg.CreateTestStaff(t, db, name[1], name[0])
	}
	texts := []string{
		"emile", "Émile Ødegård", "odegard emile", "e o", "οδυσσεασ", "οδυσσεας", "ΟΔΥΣΣΕΑΣ ΠΑΠΑ", "παπαδοπουλος οδυ",
		"παπαδοπουλοσ", "ipek", "İpek", "yildiz i", "strasser", "Straßer Jörg", "jorg s", "lena von", "heide anna",
		"aesa", "oeuvre", "œuvre æ", "li na", "na li", "2ss", "ταξησ", "ταξης", "λυκος", "λυκοσ f", "fuchse",
		"oma ab", "οδος", "οδοσ", "strasse", "eltern",
	}
	var exact, hidden int
	for _, text := range texts {
		for name, check := range map[string]struct {
			from, match, prefilter string
			args                   []any
		}{
			"children": {
				`WITH ` + liveCTE + ` SELECT count(*), count(*) FILTER (WHERE NOT ` + childPrefilter + `) FROM live ` + patternJoin + ` WHERE `,
				childNameMatch + ` OR ` + childClassMatch + ` OR ` + childGroupMatch + ` OR ` + childNoteMatch, childPrefilter,
				[]any{true, tenantID, text},
			},
			"contacts": {
				`SELECT count(*), count(*) FILTER (WHERE NOT ` + contactNamePrefilter + `) FROM users.guardian_profiles AS guardian ` + patternJoin + ` WHERE guardian.tenant_id = ? AND `,
				contactNameMatch, contactNamePrefilter, []any{text, tenantID},
			},
			"staff": {
				`SELECT count(*), count(*) FILTER (WHERE NOT ` + staffNamePrefilter + `) FROM users.persons AS person ` + patternJoin + ` WHERE person.tenant_id = ? AND `,
				`(public.f_unaccent(person.first_name || ' ' || person.last_name) ILIKE q.p ESCAPE '\'
					OR public.f_unaccent(person.last_name || ' ' || person.first_name) ILIKE q.p ESCAPE '\')`,
				staffNamePrefilter, []any{text, tenantID},
			},
		} {
			var matched, missed int
			require.NoError(t, db.NewRaw(check.from+`(`+check.match+`)`, check.args...).Scan(ctx, &matched, &missed), "%s: %s", name, text)
			require.Zero(t, missed, "%s: the prefilter hides a hit for %q", name, text)
			exact += matched
			hidden += missed
		}
	}
	require.Greater(t, exact, 2*len(texts), "the texts must find rows, or the comparison proves nothing")
	require.Zero(t, hidden)
}
