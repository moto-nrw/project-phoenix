package postgres

import "database/sql"

// ErrNoRows is the driver's "no row matched" sentinel, which the retained
// repositories return or wrap for a missing row.
var ErrNoRows = sql.ErrNoRows
