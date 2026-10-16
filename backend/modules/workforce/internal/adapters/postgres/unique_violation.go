package postgres

import (
	"errors"

	"github.com/uptrace/bun/driver/pgdriver"
)

// groupSubstitutionTransferIndex is the one unique index of
// education.group_substitution besides its primary key: a group transfer per
// group, substitute and start day.
const groupSubstitutionTransferIndex = "idx_no_duplicate_group_transfers_tenant"

// isUniqueViolationOn reports whether PostgreSQL rejected the write on the
// named unique index or constraint. A unique index raises only a
// unique_violation, so the integrity class together with the index name
// identifies it.
func isUniqueViolationOn(err error, index string) bool {
	pgErr, ok := errors.AsType[pgdriver.Error](err)
	return ok && index != "" && pgErr.IntegrityViolation() && pgErr.Field('n') == index
}
