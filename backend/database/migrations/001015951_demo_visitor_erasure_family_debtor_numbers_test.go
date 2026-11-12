package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The erasure of 1.15.951 is the erasure of 1.15.945 that erases the
// families of the visitor before it deletes the contracts of the school
// (#6812).
func TestFamilyDebtorNumberErasureIsTheErasureOf945WithTheFamilies(t *testing.T) {
	t.Parallel()
	anchor := "\t\t\t\t\tDELETE FROM care_contract.contracts\n\t\t\t\t\tWHERE school_id = ANY (hidden_schools);\n"
	addition := "\t\t\t\t\t-- The debtor numbers of the visitor's families (#6812), before\n" +
		"\t\t\t\t\t-- the contracts could detach a row a registration used.\n" +
		"\t\t\t\t\tPERFORM platform.erase_demo_visitor_families(hidden_schools);\n"
	require.Equal(t, 1, strings.Count(demoVisitorErasureParentAccountOfferSQL, anchor), "the erasure of 1.15.945 deletes the contracts exactly once")
	assert.Equal(t, strings.Replace(demoVisitorErasureParentAccountOfferSQL, anchor, addition+anchor, 1), demoVisitorErasureFamilyDebtorNumbersSQL)
}
