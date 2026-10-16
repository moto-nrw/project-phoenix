package postgres

import "github.com/moto-nrw/project-phoenix/modules/transactionruntime"

// IsUniqueViolation reports PostgreSQL's unique_violation for the retained
// schedule services through the transaction runtime's classification, which
// models/base served the composition before #4084.
func IsUniqueViolation(err error) bool { return transactionruntime.IsUniqueViolation(err) }

// IsUniqueViolationOn is IsUniqueViolation restricted to one constraint or
// index name.
func IsUniqueViolationOn(err error, constraint string) bool {
	return transactionruntime.IsUniqueViolationOn(err, constraint)
}
