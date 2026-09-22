package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// DemoDependencies are the Organisation & Tenancy facts the demo flow needs
// when the Identity & Access module is composed for the demo environment.
type DemoDependencies struct {
	Schools     DemoSchools
	Mail        identityaccess.DemoAccessMail
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
}

func composeDemoAccess(db *bun.DB, sessions DemoSessions, dependencies *DemoDependencies) (*identityaccess.DemoAccess, error) {
	if dependencies == nil {
		return nil, nil
	}
	return NewDemoAccess(DemoAccessDependencies{
		DB: db, Sessions: sessions, Schools: dependencies.Schools, Mail: dependencies.Mail,
		NewToken: dependencies.NewToken, Fingerprint: dependencies.Fingerprint,
	})
}

// DemoSessions is the session minting the demo access redeems into; the
// composed Identity & Access module satisfies it.
type DemoSessions = ports.DemoSessions

// DemoSchools reaches the demo schools through Organisation & Tenancy inside
// the flow's administrative transaction (#3463). PrepareDemoSchool returns
// the slug a new access enters: a queued school of its own or the standing
// school, or identityaccess.ErrDemoCapacityReached when no further school
// may be queued (#3466). DemoSchoolEntry reports "preparing" until the school
// is seeded, exists and is active. MarkDemoSchoolUsed notes an entry into the
// school.
type DemoSchools interface {
	PrepareDemoSchool(ctx context.Context, schoolName, personName string) (slug string, err error)
	DemoSchoolEntry(ctx context.Context, slug string) (identityaccess.DemoSchoolEntry, error)
	MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error
	// ReplaceDemoSchool hides the school and queues a fresh one with the
	// same names, returning its slug (#3470). The standing school reports
	// identityaccess.ErrDemoAccessInvalid: it is shared and never restarts.
	ReplaceDemoSchool(ctx context.Context, slug, schoolName, personName string) (newSlug string, err error)
}

type demoSchoolsPort struct{ schools DemoSchools }

func (p demoSchoolsPort) MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error {
	return p.schools.MarkDemoSchoolUsed(ctx, slug, usedAt)
}

func (p demoSchoolsPort) ReplaceDemoSchool(ctx context.Context, slug, schoolName, personName string) (string, error) {
	newSlug, err := p.schools.ReplaceDemoSchool(ctx, slug, schoolName, personName)
	switch {
	case errors.Is(err, identityaccess.ErrDemoCapacityReached):
		return "", domain.ErrDemoCapacityReached
	case errors.Is(err, identityaccess.ErrDemoAccessInvalid):
		return "", domain.ErrDemoAccessInvalid
	}
	return newSlug, err
}

func (p demoSchoolsPort) PrepareDemoSchool(ctx context.Context, schoolName, personName string) (string, error) {
	slug, err := p.schools.PrepareDemoSchool(ctx, schoolName, personName)
	if errors.Is(err, identityaccess.ErrDemoCapacityReached) {
		return "", domain.ErrDemoCapacityReached
	}
	return slug, err
}

func (p demoSchoolsPort) DemoSchoolEntry(ctx context.Context, slug string) (domain.DemoSchoolEntry, error) {
	entry, err := p.schools.DemoSchoolEntry(ctx, slug)
	return domain.DemoSchoolEntry{
		Status: entry.Status, TenantID: entry.SchoolID, AccountID: entry.AccountID, ParentAccountID: entry.ParentAccountID,
	}, err
}

// DemoAccessDependencies compose the demo access of the public demo (#3462).
// NewToken and Fingerprint are the opaque capability token of the token
// adapter, so only a SHA-256 fingerprint is stored.
type DemoAccessDependencies struct {
	DB          *bun.DB
	Sessions    DemoSessions
	Schools     DemoSchools
	Mail        identityaccess.DemoAccessMail
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
}

// NewDemoAccess composes the capability. Only the demo environment's root
// calls it. Callers supply the unit of work on the request context, as every
// public route does.
func NewDemoAccess(deps DemoAccessDependencies) (*identityaccess.DemoAccess, error) {
	if deps.DB == nil || deps.Sessions == nil || deps.Schools == nil || deps.Mail == nil || deps.NewToken == nil || deps.Fingerprint == nil {
		return nil, errors.New("identity access compose: every demo access dependency is required")
	}
	flows, err := application.NewDemoAccess(application.DemoAccessDependencies{
		Sessions: deps.Sessions, Store: newStore(deps.DB), Schools: demoSchoolsPort{schools: deps.Schools},
		Tokens:  demoAccessTokens{mint: deps.NewToken, fingerprint: deps.Fingerprint},
		Mail:    demoAccessMail{mail: deps.Mail},
		AdminTx: tenant.WithinAdmin,
	})
	if err != nil {
		return nil, err
	}
	return identityaccess.NewDemoAccess(demoAccessEngine{flows: flows}), nil
}

type demoAccessTokens struct {
	mint        func() (string, string, error)
	fingerprint func(string) string
}

func (t demoAccessTokens) NewToken() (string, string, error) { return t.mint() }

func (t demoAccessTokens) Fingerprint(raw string) string { return t.fingerprint(raw) }

// demoAccessMail hands the internal access to the root's mail binding.
type demoAccessMail struct{ mail identityaccess.DemoAccessMail }

func demoAccessMessage(access domain.DemoAccess) identityaccess.DemoAccessMessage {
	return identityaccess.DemoAccessMessage{
		AccessID: access.ID, Email: access.Email, PersonName: access.PersonName,
		SchoolName: access.SchoolName, Source: access.Source, ContactOptIn: access.ContactOptIn,
	}
}

func (m demoAccessMail) SendDemoAccessLink(ctx context.Context, access domain.DemoAccess, entryURL string) {
	m.mail.SendDemoAccessLink(ctx, demoAccessMessage(access), entryURL)
}

func (m demoAccessMail) SendDemoLead(ctx context.Context, access domain.DemoAccess) {
	m.mail.SendDemoLead(ctx, demoAccessMessage(access))
}

type demoAccessEngine struct{ flows *application.DemoAccess }

func (e demoAccessEngine) RequestDemoAccess(ctx context.Context, request identityaccess.DemoAccessRequest) error {
	return demoAccessError(e.flows.Request(ctx, domain.DemoAccess{
		Email: request.Email, PersonName: request.PersonName, SchoolName: request.SchoolName,
		Source: request.Source, ContactOptIn: request.ContactOptIn, Role: domain.DemoRole(request.Role),
	}, request.ClientIP, request.EntryURLPrefix))
}

func (e demoAccessEngine) DemoAccessStatus(ctx context.Context, token string) (identityaccess.DemoAccessProgress, error) {
	access, status, err := e.flows.Status(ctx, token)
	if err != nil {
		return identityaccess.DemoAccessProgress{}, demoAccessError(err)
	}
	return identityaccess.DemoAccessProgress{Status: status, SchoolSlug: access.SchoolSlug, SchoolName: access.SchoolName}, nil
}

func (e demoAccessEngine) RedeemDemoAccess(ctx context.Context, token, role, ipAddress, userAgent string) (identityaccess.DemoEntry, error) {
	entry, err := e.flows.Redeem(ctx, token, domain.DemoRole(role), ipAddress, userAgent)
	if err != nil {
		return identityaccess.DemoEntry{}, demoAccessError(err)
	}
	return identityaccess.DemoEntry{
		AccessToken: entry.AccessToken, RefreshToken: entry.RefreshToken,
		AccessID: entry.AccessID, Role: string(entry.Role), Source: entry.Source, FixedRole: entry.FixedRole,
	}, nil
}

func (e demoAccessEngine) ResetDemoAccess(ctx context.Context, token, clientIP string) error {
	_, err := e.flows.Reset(ctx, token, clientIP)
	return demoAccessError(err)
}

// NewDemoAccessExpiry composes the demo process's expiry (#3470) on that
// process's own connection; now is the clock the process injects.
func NewDemoAccessExpiry(db bun.IDB, now func() time.Time) (*identityaccess.DemoAccessExpiry, error) {
	if db == nil {
		return nil, errors.New("identity access compose: demo access expiry requires a database")
	}
	if now == nil {
		return nil, errors.New("identity access compose: demo access expiry requires a clock")
	}
	return identityaccess.NewDemoAccessExpiry(demoAccessExpiryEngine{store: postgres.NewDemoAccessExpiryStore(db), now: now}), nil
}

// demoAccessExpiryEngine deletes demo accesses 14 days after their last use
// (#3470). A use extends the access, so an access past its end has not been
// used for its whole lifetime; deleting it removes the prospect's contact
// data. The schools left without any access go back to the caller, which
// hides them through their owner.
type demoAccessExpiryEngine struct {
	store *postgres.DemoAccessExpiryStore
	now   func() time.Time
}

func (e demoAccessExpiryEngine) ExpireDemoAccesses(ctx context.Context) (int, []string, error) {
	return e.store.DeleteExpiredDemoAccesses(ctx, e.now())
}

var demoAccessSentinels = []struct{ internal, public error }{
	{domain.ErrDemoAccessInvalid, identityaccess.ErrDemoAccessInvalid},
	{domain.ErrDemoAccessUnknown, identityaccess.ErrDemoAccessUnknown},
	{domain.ErrDemoAccessExpired, identityaccess.ErrDemoAccessExpired},
	{domain.ErrDemoSchoolPreparing, identityaccess.ErrDemoSchoolPreparing},
	{domain.ErrDemoCapacityReached, identityaccess.ErrDemoCapacityReached},
}

func demoAccessError(err error) error {
	if err == nil {
		return nil
	}
	var limited *domain.DemoAccessRateLimitedError
	if errors.As(err, &limited) {
		return &identityaccess.DemoAccessRateLimitError{RetryAt: limited.RetryAt}
	}
	for _, sentinel := range demoAccessSentinels {
		if errors.Is(err, sentinel.internal) {
			return sentinel.public
		}
	}
	return fmt.Errorf("identity access demo access: %w", err)
}
