// Package traegerview is the public contract of the Träger projection (ADR
// 0084 section 1): the only place that reads data of several schools for the
// Träger-Portal. It reads in an administrative transaction, and every query
// filters on the schools of the Träger of the session, which Organisation &
// Tenancy supplies. A ratchet test checks that filter on every query of the
// projection's Postgres package (backend/test/traeger_view_scope_ratchet_test.go).
// The projection never writes; writes go to the school's owner in the
// school's tenant transaction.
package traegerview

import (
	"context"
	"errors"
)

// ErrTraegerRequired reports a call without the Träger of the session.
var ErrTraegerRequired = errors.New("traeger view: the Träger of the session is required")

// ErrTraegerUnavailable reports a Träger that is deactivated, deleted or
// unknown: its session reaches no school any more (#4657).
var ErrTraegerUnavailable = errors.New("traeger view: the Träger is deactivated or deleted")

// ErrSchoolIDsRequired reports composition without Organisation & Tenancy's school set.
var ErrSchoolIDsRequired = errors.New("traeger view: the school IDs of Organisation & Tenancy are required")

// School is one school of the Träger in the school list (#4649). It carries
// no personal data.
type School struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Subdomain string `json:"subdomain"`
	City      string `json:"city"`
}

// SchoolIDs resolves the schools of a Träger at Organisation & Tenancy: the
// undeleted schools of an active Träger. Any other Träger is
// ErrTraegerUnavailable. It joins the projection's administrative
// transaction.
type SchoolIDs interface {
	ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
}

// Overview answers the Träger-Portal reads.
type Overview interface {
	// Schools lists the schools of the Träger, ordered by name.
	Schools(ctx context.Context, organizationID int64) ([]School, error)
}
