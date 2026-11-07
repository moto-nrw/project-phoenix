package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// OwnTraegerDocuments removes the documents a test creates for the Träger
// (#4692) when the test ends, with their releases and reads. The rows carry
// no tenant, and the tenant cleanup deletes an emptied organization with
// foreign-key triggers suspended, so they need their own exact-key teardown
// like the master data.
func OwnTraegerDocuments(tb testing.TB, db *bun.DB, organizationID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		for _, table := range []string{"platform.traeger_document_reads", "platform.traeger_document_releases", "platform.traeger_documents"} {
			_, err := db.ExecContext(context.Background(), `DELETE FROM `+table+` WHERE organization_id = ?`, organizationID)
			require.NoError(tb, err, "Failed to clean up test Träger documents")
		}
	})
}
