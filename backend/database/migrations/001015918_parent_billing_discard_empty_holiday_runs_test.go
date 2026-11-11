package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyRunErasureChanges are the statements 1.15.918 adds to the erasure
// of 1.15.887 (#6627): each anchor of 1.15.887 with what stands there now.
var emptyRunErasureChanges = []struct{ name, before, after string }{
	{"declarations",
		`			hidden_schools BIGINT[];
`,
		`			hidden_schools BIGINT[];
			emptied_care_runs BIGINT[];
			emptied_notice_runs BIGINT[];
			emptied_holiday_runs BIGINT[];
`},
	{"monthly runs named and held",
		`					WHERE invoice.school_id = ANY (hidden_schools))
					ORDER BY run.organization_id, run.id
					FOR UPDATE;
`,
		`					WHERE invoice.school_id = ANY (hidden_schools))
					ORDER BY run.organization_id, run.id
					FOR UPDATE;
					-- A run the erasure leaves without an invoice, a Jahresbescheid
					-- or a school goes whole, also a released one (#6627): its
					-- invoices already go with the school, and a run without them
					-- would show sums of nothing. Name the runs that bill the
					-- school now; a run of other schools only loses the school.
					-- Hold the open runs of the school and the Jahresläufe too: a
					-- release or discard holds the monthly billing taken above
					-- first, so these waits end at once or with the lock timeout.
					SELECT COALESCE(array_agg(DISTINCT invoice.run_id), '{}') INTO emptied_care_runs
					FROM parent_billing.care_invoices AS invoice
					WHERE invoice.school_id = ANY (hidden_schools);
					SELECT COALESCE(array_agg(DISTINCT notice.run_id), '{}') INTO emptied_notice_runs
					FROM parent_billing.care_notices AS notice
					WHERE notice.school_id = ANY (hidden_schools);
					PERFORM 1 FROM parent_billing.care_runs AS run
					WHERE EXISTS (SELECT 1 FROM parent_billing.care_run_schools AS school
						WHERE school.organization_id = run.organization_id AND school.run_id = run.id
							AND school.school_id = ANY (hidden_schools))
					ORDER BY run.organization_id, run.id
					FOR UPDATE;
					PERFORM 1 FROM parent_billing.care_notice_runs AS run
					WHERE run.id = ANY (emptied_notice_runs) OR run.school_ids && hidden_schools
					ORDER BY run.organization_id, run.id
					FOR UPDATE;
`},
	{"monthly runs and Jahresläufe deleted",
		`					DELETE FROM care_contract.contracts
					WHERE school_id = ANY (hidden_schools);
`,
		`					DELETE FROM care_contract.contracts
					WHERE school_id = ANY (hidden_schools);
					-- The monthly runs and Jahresläufe left without an invoice, a
					-- Jahresbescheid or a school go whole, with their trail, schools,
					-- letters and due dates by cascade (#6627). A run that keeps an
					-- invoice or a Jahresbescheid never goes. An open run of other
					-- schools too only loses the retired school; a released or
					-- discarded one keeps it and its sums, as it never changes.
					DELETE FROM parent_billing.care_runs AS run
					WHERE NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices AS invoice
							WHERE invoice.organization_id = run.organization_id AND invoice.run_id = run.id)
						AND (run.id = ANY (emptied_care_runs)
							OR (EXISTS (SELECT 1 FROM parent_billing.care_run_schools AS school
									WHERE school.organization_id = run.organization_id AND school.run_id = run.id
										AND school.school_id = ANY (hidden_schools))
								AND NOT EXISTS (SELECT 1 FROM parent_billing.care_run_schools AS school
									WHERE school.organization_id = run.organization_id AND school.run_id = run.id
										AND NOT (school.school_id = ANY (hidden_schools)))));
					DELETE FROM parent_billing.care_run_schools AS school
					USING parent_billing.care_runs AS run
					WHERE school.school_id = ANY (hidden_schools)
						AND run.organization_id = school.organization_id
						AND run.id = school.run_id
						AND run.status = 'calculated';
					DELETE FROM parent_billing.care_notice_runs AS run
					WHERE NOT EXISTS (SELECT 1 FROM parent_billing.care_notices AS notice
							WHERE notice.organization_id = run.organization_id AND notice.run_id = run.id)
						AND (run.id = ANY (emptied_notice_runs)
							OR (cardinality(run.school_ids) > 0 AND run.school_ids <@ hidden_schools));
					UPDATE parent_billing.care_notice_runs AS run
					SET school_ids = ARRAY(SELECT kept.id
						FROM unnest(run.school_ids) WITH ORDINALITY AS kept(id, position)
						WHERE NOT (kept.id = ANY (hidden_schools))
						ORDER BY kept.position)
					WHERE run.status = 'calculated' AND run.school_ids && hidden_schools;
`},
	{"holiday runs named",
		`					DELETE FROM parent_billing.documents AS document
					USING enrollment.holiday_registration_children AS child
`,
		`					-- The holiday runs that billed the registrations; a run left
					-- without an invoice goes whole below (#6627).
					SELECT COALESCE(array_agg(DISTINCT document.run_id), '{}') INTO emptied_holiday_runs
					FROM parent_billing.documents AS document
					JOIN enrollment.holiday_registration_children AS child
						ON child.organization_id = document.organization_id
						AND child.holiday_registration_id = document.holiday_registration_id
					WHERE child.home_school_id = ANY (hidden_schools);
					DELETE FROM parent_billing.documents AS document
					USING enrollment.holiday_registration_children AS child
`},
	{"holiday runs deleted",
		`						AND document.holiday_registration_id = child.holiday_registration_id;
					-- The receipt of the registration went to the guardian's
`,
		`						AND document.holiday_registration_id = child.holiday_registration_id;
					-- A released holiday run whose invoices all went goes whole,
					-- with its trail, letters and due dates by cascade (#6627); one
					-- that keeps an invoice of another family stays as it is.
					DELETE FROM parent_billing.runs AS run
					WHERE run.id = ANY (emptied_holiday_runs)
						AND NOT EXISTS (SELECT 1 FROM parent_billing.documents AS document
							WHERE document.organization_id = run.organization_id AND document.run_id = run.id);
					-- The receipt of the registration went to the guardian's
`},
}

// The erasure of 1.15.918 is the erasure of 1.15.887 with the statements
// that take the emptied runs (#6627), and nothing else changed.
func TestEmptyRunErasureIsTheErasureOf887WithTheRuns(t *testing.T) {
	t.Parallel()
	want := demoVisitorErasureHolidayClosureSQL
	for _, change := range emptyRunErasureChanges {
		require.Equal(t, 1, strings.Count(want, change.before), "%s: the anchor is in the erasure of 1.15.887 exactly once", change.name)
		want = strings.Replace(want, change.before, change.after, 1)
	}
	assert.Equal(t, want, demoVisitorErasureEmptyRunsSQL)
	assert.Contains(t, demoVisitorErasureEmptyRunsSQL, "DELETE FROM parent_billing.runs AS run")
}
