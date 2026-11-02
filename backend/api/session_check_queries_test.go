package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// withoutSessionCheck removes the session family check every authenticated
// request runs before its route (#5194) and proves its shape on the way: one
// administrative transaction with one read of the family's newest
// generation. Its cost has a budget of its own
// (identity.access_session.verify); router budgets count their route.
func withoutSessionCheck(t *testing.T, queries []string) []string {
	t.Helper()
	for i := 2; i+1 < len(queries); i++ {
		if !strings.Contains(queries[i], `"auth"."tokens"`) || !strings.Contains(queries[i], "family_id") {
			continue
		}
		block := queries[i-2 : i+2]
		require.Equal(t, "BEGIN", block[0], "the session check reads in a transaction of its own")
		require.Contains(t, block[1], "SET LOCAL ROLE phoenix_admin", "the session check reads across schools")
		require.Contains(t, block[2], "generation", "the session check reads the newest generation")
		require.Equal(t, "COMMIT", block[3], "the session check reads nothing but the family")
		rest := append(append([]string{}, queries[:i-2]...), queries[i+2:]...)
		for _, query := range rest {
			require.Falsef(t, strings.Contains(query, `"auth"."tokens"`), "the session check must run once per request: %s", query)
		}
		return rest
	}
	require.Fail(t, "the request ran no session family check", strings.Join(queries, "\n"))
	return nil
}
