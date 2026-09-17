package auth

import "context"

// InvitationSchool is the part of a school the invitation flows read.
type InvitationSchool struct {
	Name      string
	Slug      string
	Subdomain string
	// Settings is the school's raw settings JSON; the guardian invitation
	// reads the login image from it.
	Settings string
	Deleted  bool
}

// SchoolDirectory reads the school an invitation belongs to. Organisation &
// Tenancy owns the row; the root binds this port to its capability. A school
// that does not exist is (nil, nil).
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (*InvitationSchool, error)
	// FindSchoolForShare also takes a shared lock on the school row that
	// holds until the caller's transaction ends, so a concurrent soft delete
	// cannot commit in between.
	FindSchoolForShare(ctx context.Context, id int64) (*InvitationSchool, error)
}
