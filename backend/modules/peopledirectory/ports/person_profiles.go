package ports

import "context"

// LiveStaffProfiles is the School Membership seam a person's retirement asks
// before it soft-deletes the row. The staff membership belongs to that owner,
// so the composition root binds it; the directory never reads users.staff for
// this check itself (#6840).
type LiveStaffProfiles interface {
	// FindStaffIDByPerson returns the person's live staff membership at the
	// school of the caller's transaction; found is false for none. A retired
	// membership does not count.
	FindStaffIDByPerson(ctx context.Context, personID int64) (staffID int64, found bool, err error)
}
