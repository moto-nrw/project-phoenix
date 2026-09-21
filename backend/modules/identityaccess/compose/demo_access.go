package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
}

func composeDemoAccess(db *bun.DB, sessions DemoSessions, dependencies *DemoDependencies) (*identityaccess.DemoAccess, error) {
	if dependencies == nil {
		return nil, nil
	}
	return NewDemoAccess(DemoAccessDependencies{
		DB: db, Sessions: sessions, Schools: dependencies.Schools,
		NewToken: dependencies.NewToken, Fingerprint: dependencies.Fingerprint,
	})
}

// DemoSessions is the session minting the demo access redeems into; the
// composed Identity & Access module satisfies it.
type DemoSessions = ports.DemoSessions

// DemoSchools resolves the demo school by slug through Organisation &
// Tenancy; found is false until the school exists and is active.
type DemoSchools = ports.DemoSchools

// DemoAccessDependencies compose the demo access of the public demo (#3462).
// NewToken and Fingerprint are the opaque capability token of the token
// adapter, so only a SHA-256 fingerprint is stored.
type DemoAccessDependencies struct {
	DB          *bun.DB
	Sessions    DemoSessions
	Schools     DemoSchools
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
}

// NewDemoAccess composes the capability. Only the demo environment's root
// calls it. Callers supply the unit of work on the request context, as every
// public route does.
func NewDemoAccess(deps DemoAccessDependencies) (*identityaccess.DemoAccess, error) {
	if deps.DB == nil || deps.Sessions == nil || deps.Schools == nil || deps.NewToken == nil || deps.Fingerprint == nil {
		return nil, errors.New("identity access compose: every demo access dependency is required")
	}
	flows, err := application.NewDemoAccess(application.DemoAccessDependencies{
		Sessions: deps.Sessions, Store: newStore(deps.DB), Schools: deps.Schools,
		Tokens:  demoAccessTokens{mint: deps.NewToken, fingerprint: deps.Fingerprint},
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

type demoAccessEngine struct{ flows *application.DemoAccess }

func (e demoAccessEngine) RequestDemoAccess(ctx context.Context, request identityaccess.DemoAccessRequest) (identityaccess.IssuedDemoAccess, error) {
	id, token, err := e.flows.Request(ctx, domain.DemoAccess{
		Email: request.Email, PersonName: request.PersonName, SchoolName: request.SchoolName,
		Source: request.Source, ContactOptIn: request.ContactOptIn,
	})
	if err != nil {
		return identityaccess.IssuedDemoAccess{}, demoAccessError(err)
	}
	return identityaccess.IssuedDemoAccess{ID: id, Token: token}, nil
}

func (e demoAccessEngine) DemoAccessReady(ctx context.Context, token, schoolSlug string) (bool, error) {
	ready, err := e.flows.Ready(ctx, token, schoolSlug)
	return ready, demoAccessError(err)
}

func (e demoAccessEngine) RedeemDemoAccess(ctx context.Context, token, schoolSlug, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := e.flows.Redeem(ctx, token, schoolSlug, ipAddress, userAgent)
	return access, refresh, demoAccessError(err)
}

var demoAccessSentinels = []struct{ internal, public error }{
	{domain.ErrDemoAccessInvalid, identityaccess.ErrDemoAccessInvalid},
	{domain.ErrDemoAccessUnknown, identityaccess.ErrDemoAccessUnknown},
	{domain.ErrDemoAccessExpired, identityaccess.ErrDemoAccessExpired},
	{domain.ErrDemoSchoolPreparing, identityaccess.ErrDemoSchoolPreparing},
}

func demoAccessError(err error) error {
	if err == nil {
		return nil
	}
	for _, sentinel := range demoAccessSentinels {
		if errors.Is(err, sentinel.internal) {
			return sentinel.public
		}
	}
	return fmt.Errorf("identity access demo access: %w", err)
}
