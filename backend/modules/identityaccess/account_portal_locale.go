package identityaccess

import "context"

// AccountPortalLocales stores the account holder's parents-portal language,
// independently of school profiles. The caller authorizes the account ID.
// Operations require the caller's administrative transaction; they never
// open or commit a separate unit.
// This capability is composed with account profiles, outside the Owner facade.
type AccountPortalLocales interface {
	FindAccountPortalLocale(context.Context, int64) (*string, error)
	SetAccountPortalLocale(context.Context, int64, string) error
}
