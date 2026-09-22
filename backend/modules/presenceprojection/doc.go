// Package presenceprojection is the tenant-safe read projection that joins
// the Timetable plan of a block and its participants with the Student
// Presence execution and attendance after the presence cutover (#2762).
// It serves the retained legacy consumers that still read one row per block
// or participant; owners never write through it.
package presenceprojection

import "errors"

var ErrInvalidTenantID = errors.New("presence projection: tenant ID must be positive")
