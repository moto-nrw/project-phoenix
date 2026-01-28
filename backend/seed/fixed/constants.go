package fixed

// SQL Upsert Patterns
// These constants reduce duplication across seed files and ensure consistency
// in how we handle CONFLICT updates and RETURNING clauses.

const (
	// SQLExcludedUpdatedAt is used in ON CONFLICT...SET clauses to update the timestamp
	SQLExcludedUpdatedAt = "updated_at = EXCLUDED.updated_at"

	// SQLBaseColumns is used in RETURNING clauses for base model fields
	SQLBaseColumns = "id, created_at, updated_at"
)
