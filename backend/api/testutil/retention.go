package testutil

import (
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// RetentionTestOwners are the retained owners the Worker's nightly retention
// steps bind, composed over the test database (#3950).
type RetentionTestOwners = services.RetentionTestOwners

// NewRetentionTestOwners composes the retention owners over db for the root's
// retention suite.
func NewRetentionTestOwners(db *bun.DB) (RetentionTestOwners, error) {
	return services.NewRetentionTestOwners(db)
}
