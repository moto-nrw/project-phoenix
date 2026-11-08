package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// mealBillingErasureStatement is what 1.15.811 adds to the erasure of 1.15.809; 1.15.823
// anchors on its end.
const mealBillingErasureStatement = `					-- A Mensa month a school released keeps its children with
					-- their meals (#4686); it goes with the school.
					DELETE FROM schedule.meal_billing_months
					WHERE school_id = ANY (hidden_schools);
					-- How far the retention deleted its attendance (#4686).
					DELETE FROM active.attendance_retention_marks
					WHERE school_id = ANY (hidden_schools);
`

// careNoticeErasureAnchor is the end of what 1.15.811 erases of the Mensa;
// the contracts of the demo school still exist there.
const careNoticeErasureAnchor = mealBillingErasureAnchor + mealBillingErasureStatement

// careNoticeErasureStatement is what 1.15.823 adds to the erasure of
// 1.15.821 (#6219), which is the erasure of 1.15.811 with the changes of the
// review (#6197). A Jahresbescheid copies child, recipient and mail
// address and outlives its contract; nothing removes it with the school,
// because it belongs to the Träger. The notices of the school go, their
// months with the mail addresses of the snapshot by cascade, also a notice
// that names nobody: the school decides, not the name. A read in the trail
// of a notice run names the contracts it showed; it goes while those
// contracts still exist. The function holds the monthly billing of the
// Träger before, so no release of a notice run is under way.
const careNoticeErasureStatement = `					-- A Jahresbescheid of the school copies child, recipient and
					-- mail address (#6219); it goes with the school, its months
					-- by cascade, whether or not it names anybody.
					DELETE FROM parent_billing.care_notices
					WHERE school_id = ANY (hidden_schools);
					DELETE FROM parent_billing.care_notice_run_events AS event
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND event.organization_id = contract.organization_id
						AND event.details->'contract_ids' @> to_jsonb(contract.id);
`

// demoVisitorErasureCareNoticesSQL is platform.erase_closed_demo_visitors
// of 1.15.821 with the fee notices of the demo schools.
var demoVisitorErasureCareNoticesSQL = strings.Replace(demoVisitorErasureCareAmendmentsSQL, careNoticeErasureAnchor,
	careNoticeErasureAnchor+careNoticeErasureStatement, 1)

// installCareNoticeErasure installs the erasure of 1.15.823.
func installCareNoticeErasure(ctx context.Context, db *bun.DB) error {
	if strings.Count(demoVisitorErasureCareAmendmentsSQL, careNoticeErasureAnchor) != 1 {
		return fmt.Errorf("erase the fee notices of retired demo schools: the erasure of 1.15.821 lost its anchor")
	}
	if _, err := db.ExecContext(ctx, demoVisitorErasureCareNoticesSQL); err != nil {
		return fmt.Errorf("erase the fee notices of retired demo schools: %w", err)
	}
	return nil
}
