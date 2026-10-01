package identityaccess

import (
	"context"
	"errors"
	"math"
	"time"
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
	// ErrDemoAccessRateLimited reports a request over the limit of its
	// address or its IP address (#3466); DemoAccessRateLimitError carries it.
	ErrDemoAccessRateLimited = errors.New("too many demo access requests")
	// ErrDemoCapacityReached reports that no further demo school may be
	// created while the configured number of demo schools exists (#3466).
	ErrDemoCapacityReached = errors.New("demo capacity reached")
)

// DemoAccessRateLimitError reports when the limited caller may ask again.
type DemoAccessRateLimitError struct{ RetryAt time.Time }

func (e *DemoAccessRateLimitError) Error() string { return ErrDemoAccessRateLimited.Error() }

func (e *DemoAccessRateLimitError) Unwrap() error { return ErrDemoAccessRateLimited }

// RetryAfterSeconds is the wait until the next accepted request, rounded up
// and at least one second, for the Retry-After header.
func (e *DemoAccessRateLimitError) RetryAfterSeconds(now time.Time) int {
	return max(1, int(math.Ceil(e.RetryAt.Sub(now).Seconds())))
}

// DemoAccessRequest carries what a prospect submits.
type DemoAccessRequest struct {
	Email        string
	FirstName    string
	LastName     string
	SchoolName   string
	Source       string
	ContactOptIn bool
	// Role preselects a demo role in the mailed link (#3467); empty chooses none.
	Role string
	// ClientIP is the address the request came from; the requests of one IP
	// address are limited (#3466).
	ClientIP string
	// EntryURLPrefix precedes the token in the link the prospect receives.
	EntryURLPrefix string
}

// Progress of the demo school behind a demo access.
const (
	DemoSchoolPreparing = "preparing"
	DemoSchoolReady     = "ready"
	DemoSchoolFailed    = "failed"
)

// DemoSchoolEntry is what Organisation & Tenancy reports about a demo school.
// AccountID is the prospect's own caregiver; zero signs in the school's
// administrator, as in the standing demo school.
type DemoSchoolEntry struct {
	Status    string
	SchoolID  int64
	AccountID int64
	// ParentAccountID is the parent the demo role parent signs in (#3468).
	ParentAccountID int64
}

// DemoAccessMessage is what the two demo mails (#3465) say about an access.
type DemoAccessMessage struct {
	AccessID     int64
	Email        string
	FirstName    string
	LastName     string
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
	RequestDemoAccess(ctx context.Context, request DemoAccessRequest) error
	DemoAccessStatus(ctx context.Context, token string) (DemoAccessProgress, error)
	RedeemDemoAccess(ctx context.Context, token, role, ipAddress, userAgent string) (DemoEntry, error)
	ResetDemoAccess(ctx context.Context, token, clientIP string) error
}

// DemoAccessExpiryEngine is the composed implementation behind DemoAccessExpiry.
type DemoAccessExpiryEngine interface {
	ExpireDemoAccesses(ctx context.Context) (deleted int, orphanedSchoolSlugs []string, err error)
}

// DemoAccessExpiry is the demo process's capability (#3470): it deletes demo
// accesses 14 days after their last use and names the demo schools that no
// access enters any more, so the process can hide them through their owner.
type DemoAccessExpiry struct{ engine DemoAccessExpiryEngine }

func NewDemoAccessExpiry(engine DemoAccessExpiryEngine) *DemoAccessExpiry {
	if engine == nil {
		panic("demo access expiry requires an engine")
	}
	return &DemoAccessExpiry{engine: engine}
}

// ExpireDemoAccesses deletes the accesses past their end and returns how
// many, plus the schools left without any access.
func (e *DemoAccessExpiry) ExpireDemoAccesses(ctx context.Context) (int, []string, error) {
	return e.engine.ExpireDemoAccesses(ctx)
}

// DemoEntry is a redeemed demo access (#3467): the token pair and what the
// demo banner shows and reports. AccessID identifies the demo access in the
// product analytics instead of a person; Role is the demo role the session
// really has, empty when none was chosen. FixedRole marks the standing
// school, whose shared administrator no switch changes.
type DemoEntry struct {
	AccessToken  string
	RefreshToken string
	AccessID     int64
	Role         string
	Source       string
	FixedRole    bool
}

// DemoAccessProgress is what the token's holder may know about its demo
// school: the progress, the slug that is the school's subdomain once it is
// ready, and the OGS name the prospect gave, which the waiting room shows
// (#3464).
type DemoAccessProgress struct {
	Status     string
	SchoolSlug string
	SchoolName string
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

func (d *DemoAccess) RequestDemoAccess(ctx context.Context, request DemoAccessRequest) error {
	return d.engine.RequestDemoAccess(ctx, request)
}

// DemoAccessStatus reports the progress of the token's demo school.
func (d *DemoAccess) DemoAccessStatus(ctx context.Context, token string) (DemoAccessProgress, error) {
	return d.engine.DemoAccessStatus(ctx, token)
}

// RedeemDemoAccess mints a session in the token's demo school, in the chosen
// demo role: a tenant session for caregiver, lead or all (empty keeps the
// account's role), a parents portal session for parent (#3468).
func (d *DemoAccess) RedeemDemoAccess(ctx context.Context, token, role, ipAddress, userAgent string) (DemoEntry, error) {
	return d.engine.RedeemDemoAccess(ctx, token, role, ipAddress, userAgent)
}

// ResetDemoAccess gives the token's access a fresh demo school and hides the
// old one (#3470). The same token then enters the new school once it is
// seeded. ErrDemoSchoolPreparing while the current school is still being
// prepared; ErrDemoAccessInvalid for the shared standing school. A restart
// counts against the request limits of the address and of clientIP (#3466).
func (d *DemoAccess) ResetDemoAccess(ctx context.Context, token, clientIP string) error {
	return d.engine.ResetDemoAccess(ctx, token, clientIP)
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
