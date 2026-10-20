package identityaccess

import (
	"context"
	"time"
)

// AccountMetadata is display information, without credentials or ORM behavior.
type AccountMetadata struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Email         string
	Username      *string
	Avatar        string
	Active        bool
	IsPasswordOTP bool
	LastLogin     *time.Time
}

// AccountProfiles reads and edits profile fields. Bio/settings are school-scoped
// and require a tenant; account metadata, username and avatar are global. The
// caller authorizes the account (the authenticated subject or a linked person).
// Every operation joins the caller's transaction; no credentials leave the owner.
type AccountProfiles interface {
	FindAccountMetadata(context.Context, int64) (AccountMetadata, error)
	SetAccountUsername(context.Context, int64, string) error
	SetAccountAvatar(context.Context, int64, string) error
	FindAccountProfile(context.Context, int64) (bio, settings string, found bool, err error)
	SetAccountBio(context.Context, int64, string) error
}
