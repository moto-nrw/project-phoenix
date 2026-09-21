package identityaccess

import (
	"context"
	"errors"
)

// Demo access (#3462): a prospect of the public demo asks for an access,
// receives an opaque token, and redeems it for a session in the demo school
// without a password. Only the token's SHA-256 fingerprint is stored; the
// token opens the demo for 14 days as often as it is used, and every use is
// noted. The capability is composed in the demo environment only.
var (
	ErrDemoAccessInvalid = errors.New("demo access request is invalid")
	ErrDemoAccessUnknown = errors.New("demo access is unknown")
	ErrDemoAccessExpired = errors.New("demo access has expired")
	// ErrDemoSchoolPreparing reports a demo school nobody can enter yet.
	ErrDemoSchoolPreparing = errors.New("demo school is being prepared")
	// ErrDemoSessionTenantLocked rejects the tenant switch of an account a
	// demo access signed in; SwitchTenant reports it in its error envelope.
	ErrDemoSessionTenantLocked = errors.New("demo sessions cannot switch tenants")
)

// DemoAccessRequest carries what a prospect submits.
type DemoAccessRequest struct {
	Email        string
	PersonName   string
	SchoolName   string
	Source       string
	ContactOptIn bool
	// EntryURLPrefix precedes the token in the link the prospect receives.
	EntryURLPrefix string
}

// IssuedDemoAccess is a stored demo access with its token. The token exists
// only here; it cannot be read back. LinkSent reports an address that already
// had an active access: its link went out by mail only and Token is empty.
type IssuedDemoAccess struct {
	ID       int64
	Token    string
	LinkSent bool
}

// DemoAccessMessage is what the two demo mails (#3465) say about an access.
type DemoAccessMessage struct {
	AccessID     int64
	Email        string
	PersonName   string
	SchoolName   string
	Source       string
	ContactOptIn bool
}

// DemoAccessMail sends the demo mails without blocking the request.
type DemoAccessMail interface {
	SendDemoAccessLink(ctx context.Context, message DemoAccessMessage, entryURL string)
	SendDemoLead(ctx context.Context, message DemoAccessMessage)
}

// DemoAccessEngine is the composed implementation behind DemoAccess.
type DemoAccessEngine interface {
	RequestDemoAccess(ctx context.Context, request DemoAccessRequest) (IssuedDemoAccess, error)
	DemoAccessReady(ctx context.Context, token, schoolSlug string) (bool, error)
	RedeemDemoAccess(ctx context.Context, token, schoolSlug, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
}

// DemoAccess is the capability the public demo routes consume, separate from
// the module every environment composes.
type DemoAccess struct{ engine DemoAccessEngine }

func NewDemoAccess(engine DemoAccessEngine) *DemoAccess {
	if engine == nil {
		panic("demo access requires an engine")
	}
	return &DemoAccess{engine: engine}
}

func (d *DemoAccess) RequestDemoAccess(ctx context.Context, request DemoAccessRequest) (IssuedDemoAccess, error) {
	return d.engine.RequestDemoAccess(ctx, request)
}

// DemoAccessReady reports whether the school with the slug can be entered.
func (d *DemoAccess) DemoAccessReady(ctx context.Context, token, schoolSlug string) (bool, error) {
	return d.engine.DemoAccessReady(ctx, token, schoolSlug)
}

// RedeemDemoAccess mints a tenant session in the school with the slug.
func (d *DemoAccess) RedeemDemoAccess(ctx context.Context, token, schoolSlug, ipAddress, userAgent string) (string, string, error) {
	return d.engine.RedeemDemoAccess(ctx, token, schoolSlug, ipAddress, userAgent)
}

// NewDemoModule is NewModule with the demo-only capability; access is nil
// outside the demo environment.
func NewDemoModule(engine Engine, runtime TenantRuntimeBinding, access *DemoAccess) *Module {
	module := NewModule(engine, runtime)
	module.demoAccess = access
	return module
}

// DemoAccess returns the demo-only capability when this module was composed
// in the demo environment.
func (m *Module) DemoAccess() *DemoAccess { return m.demoAccess }
