//go:build testhooks

package api

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
)

// ClassList12Router builds the production class-list route over its existing
// test composition, including the real People Directory and Audit bindings.
func ClassList12Router(db transactionruntime.Database) (chi.Router, error) {
	module, err := NewClassListTestModule(db)
	if err != nil {
		return nil, err
	}
	return newClassListEntriesResource(module.Membership, slog.Default()).Router(), nil
}
