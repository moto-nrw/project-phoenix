package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The release of a holiday run writes on each position the KOST1 of the
// school that hosts its week, read through Enrollment, valid on the first
// day of the week; without one it writes the holiday care's with the reason
// (#6624). A later KOST1 changes no issued position.
func TestHolidayReleaseBooksEachWeekOnItsHostSchool(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Gastgeber-KOST1")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	ctx := context.Background()
	addCenter := func(scope string, scopeID int64, from, code string) {
		t.Helper()
		_, err := f.db.ExecContext(ctx, `INSERT INTO parent_billing.cost_centers (organization_id, scope, scope_id, valid_from, code)
			VALUES (?, ?, ?, ?::date, ?)`, f.office.organization, scope, scopeID, from, code)
		require.NoError(t, err)
	}
	// The first week (4 January) is hosted by the first school, the second
	// (11 January) by the second, whose KOST1 starts only on 12 January.
	addCenter("school", f.office.schools[0], "2027-01-01", "ISP")
	addCenter("school", f.office.schools[1], "2027-01-12", "BIR")
	addCenter("holiday", 0, "2026-01-01", "FER")

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := startBillingRun(t, f, billing, f.period.ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(billingRun(t, f, billing, run.ID)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	type position struct {
		WeekStartsOn *string `bun:"week_starts_on"`
		CostCenter   *string `bun:"cost_center"`
		Fallback     *string `bun:"cost_center_fallback"`
		Frozen       bool    `bun:"accounts_frozen"`
	}
	read := func() []position {
		var positions []position
		require.NoError(t, f.db.NewRaw(`SELECT position.week_starts_on::text AS week_starts_on, position.cost_center,
			position.cost_center_fallback, position.accounts_frozen
			FROM parent_billing.document_positions AS position
			JOIN parent_billing.documents AS document ON document.id = position.document_id
			WHERE document.run_id = ? AND position.kind = 'base' ORDER BY position.week_starts_on, position.id`, run.ID).Scan(ctx, &positions))
		return positions
	}
	positions := read()
	require.Len(t, positions, 3, "Lia books the first week, Ben both")
	text := func(value *string) string {
		if value == nil {
			return ""
		}
		return *value
	}
	var got [][3]string
	for _, p := range positions {
		assert.True(t, p.Frozen)
		got = append(got, [3]string{text(p.WeekStartsOn), text(p.CostCenter), text(p.Fallback)})
	}
	assert.Equal(t, [][3]string{
		{"2027-01-04", "ISP", ""},
		{"2027-01-04", "ISP", ""},
		{"2027-01-11", "FER", "no_school_cost_center"},
	}, got)

	// A KOST1 entered after the release reaches no issued position.
	addCenter("school", f.office.schools[1], "2027-01-01", "BIR0")
	assert.Equal(t, positions, read())
}
