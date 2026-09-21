package identityaccess

import "context"

// SchoolAccountListing contains identity-owned facts for operator account lists.
// AccountID zero with status "invited" denotes a pending invitation; names
// belong to that invitation. Person names and caregiver facts belong to other owners.
type SchoolAccountListing struct {
	SchoolID     int64
	AccountID    int64
	Email        string
	Active       bool
	FirstName    string
	LastName     string
	RoleName     string
	Status       string
	HasAdminRole bool
	HasUserRole  bool
}

// SchoolAccountListings reads only the explicit school set. The caller owns
// school visibility; the query preserves its ambient transaction and RLS.
type SchoolAccountListings interface {
	ListSchoolAccountListings(context.Context, []int64) ([]SchoolAccountListing, error)
}

func (m *Module) ListSchoolAccountListings(ctx context.Context, schoolIDs []int64) ([]SchoolAccountListing, error) {
	return m.engine.ListSchoolAccountListings(ctx, schoolIDs)
}
