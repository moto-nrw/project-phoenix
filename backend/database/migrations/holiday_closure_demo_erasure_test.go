package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHolidayClosureDemoErasurePreserves927(t *testing.T) {
	t.Parallel()
	const anchor = "\t\t\t\tIF cardinality(visitor.visitor_account_ids) > 0 OR cardinality(visitor.visitor_parent_account_ids) > 0 THEN"
	const addition = "\t\t\t\t-- Approval history is personal data; its proposal owns both cascades.\n" +
		"\t\t\t\tDELETE FROM calendar.holiday_closure_proposals\n" +
		"\t\t\t\tWHERE tenant_id = ANY (hidden_schools);\n"
	require.Equal(t, 1, strings.Count(demoVisitorErasureHolidayAcceptanceSQL, anchor))
	require.Equal(t, strings.Replace(demoVisitorErasureHolidayAcceptanceSQL, anchor, addition+anchor, 1), demoVisitorErasureHolidayClosuresSQL)
}
