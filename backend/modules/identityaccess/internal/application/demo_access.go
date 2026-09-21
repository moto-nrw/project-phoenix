package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// DemoAccess runs the demo access of the public demo (#3462): a prospect
// asks for an access, receives an opaque token, and redeems it for a session
// in the demo school as often as it likes within its lifetime. The session
// is minted by the account authentication; nothing here signs a token.
type DemoAccess struct {
	sessions ports.DemoSessions
	store    ports.DemoAccessStore
	schools  ports.DemoSchools
	tokens   ports.DemoAccessTokens
	adminTx  ports.DemoAdminTx
	now      func() time.Time
}

// DemoAccessDependencies are the ports the flows consume.
type DemoAccessDependencies struct {
	Sessions ports.DemoSessions
	Store    ports.DemoAccessStore
	Schools  ports.DemoSchools
	Tokens   ports.DemoAccessTokens
	AdminTx  ports.DemoAdminTx
}

func NewDemoAccess(deps DemoAccessDependencies) (*DemoAccess, error) {
	if deps.Sessions == nil || deps.Store == nil || deps.Schools == nil || deps.Tokens == nil || deps.AdminTx == nil {
		return nil, fmt.Errorf("identity access demo access: every dependency is required")
	}
	return &DemoAccess{
		sessions: deps.Sessions, store: deps.Store, schools: deps.Schools, tokens: deps.Tokens, adminTx: deps.AdminTx, now: time.Now,
	}, nil
}

// Request stores a new demo access with the school it enters and returns its
// token. The token is returned once and never stored. An address that still
// has an active access gets the zero value: no second school and no token.
func (d *DemoAccess) Request(ctx context.Context, access domain.DemoAccess) (domain.IssuedDemoAccess, error) {
	if err := access.Normalize(); err != nil {
		return domain.IssuedDemoAccess{}, err
	}
	raw, fingerprint, err := d.tokens.NewToken()
	if err != nil {
		return domain.IssuedDemoAccess{}, fmt.Errorf("mint demo access token: %w", err)
	}
	access.TokenHash = fingerprint
	access.ExpiresAt = d.now().Add(domain.DemoAccessLifetime)
	var id int64
	var active bool
	err = d.adminTx(ctx, func(txCtx context.Context) error {
		var txErr error
		if active, txErr = d.hasActiveAccess(txCtx, access.Email); txErr != nil || active {
			return txErr
		}
		if access.SchoolSlug, txErr = d.schools.PrepareDemoSchool(txCtx, access.SchoolName, access.PersonName); txErr != nil {
			return txErr
		}
		id, txErr = d.store.InsertDemoAccess(txCtx, access)
		return txErr
	})
	if err != nil {
		return domain.IssuedDemoAccess{}, fmt.Errorf("store demo access: %w", err)
	}
	if active {
		return domain.IssuedDemoAccess{}, nil
	}
	return domain.IssuedDemoAccess{ID: id, Token: raw, SchoolSlug: access.SchoolSlug}, nil
}

// hasActiveAccess reports an unexpired access of the address whose school did
// not fail. Such an address gets no second school (#3463).
func (d *DemoAccess) hasActiveAccess(ctx context.Context, email string) (bool, error) {
	if err := d.store.LockDemoAccessEmail(ctx, email); err != nil {
		return false, err
	}
	slugs, err := d.store.UnexpiredDemoAccessSchools(ctx, email, d.now())
	if err != nil {
		return false, err
	}
	for _, slug := range slugs {
		entry, err := d.schools.DemoSchoolEntry(ctx, slug)
		if err != nil {
			return false, err
		}
		if entry.Status != domain.DemoSchoolFailed {
			return true, nil
		}
	}
	return false, nil
}

// Status reports the progress of the token's demo school and its slug, which
// is the school's address once it is ready. A ready school without an account
// to sign in is still preparing.
func (d *DemoAccess) Status(ctx context.Context, token string) (status, schoolSlug string, err error) {
	err = d.adminTx(ctx, func(txCtx context.Context) error {
		access, err := d.valid(txCtx, token)
		if err != nil {
			return err
		}
		entry, err := d.entry(txCtx, access.SchoolSlug)
		status, schoolSlug = entry.Status, access.SchoolSlug
		return err
	})
	return status, schoolSlug, err
}

// Redeem notes the use and mints a session in the token's demo school: for
// the prospect's own caregiver, or for the administrator of a school without
// one. The token stays valid afterwards.
func (d *DemoAccess) Redeem(ctx context.Context, token, ipAddress, userAgent string) (string, string, error) {
	var entry domain.DemoSchoolEntry
	err := d.adminTx(ctx, func(txCtx context.Context) error {
		access, err := d.valid(txCtx, token)
		if err != nil {
			return err
		}
		if entry, err = d.entry(txCtx, access.SchoolSlug); err != nil {
			return err
		}
		if entry.Status != domain.DemoSchoolReady {
			return domain.ErrDemoSchoolPreparing
		}
		return d.store.RecordDemoAccessUse(txCtx, access.ID, entry.AccountID, d.now())
	})
	if err != nil {
		return "", "", err
	}
	return d.sessions.IssueTokensForAuthenticatedAccount(ctx, entry.AccountID, entry.TenantID, ipAddress, userAgent)
}

// entry resolves the account a demo session signs in.
func (d *DemoAccess) entry(ctx context.Context, schoolSlug string) (domain.DemoSchoolEntry, error) {
	entry, err := d.schools.DemoSchoolEntry(ctx, schoolSlug)
	if err != nil || entry.Status != domain.DemoSchoolReady || entry.AccountID != 0 {
		return entry, err
	}
	accountID, found, err := d.store.FindSchoolAdministrator(ctx, entry.TenantID)
	if err != nil {
		return domain.DemoSchoolEntry{}, err
	}
	if !found {
		entry.Status = domain.DemoSchoolPreparing
	}
	entry.AccountID = accountID
	return entry, nil
}

func (d *DemoAccess) valid(ctx context.Context, token string) (domain.DemoAccess, error) {
	if token == "" {
		return domain.DemoAccess{}, domain.ErrDemoAccessUnknown
	}
	access, found, err := d.store.FindDemoAccessByTokenHash(ctx, d.tokens.Fingerprint(token))
	if err != nil {
		return domain.DemoAccess{}, err
	}
	if !found {
		return domain.DemoAccess{}, domain.ErrDemoAccessUnknown
	}
	if access.Expired(d.now()) {
		return domain.DemoAccess{}, domain.ErrDemoAccessExpired
	}
	return access, nil
}

// demoTenantLock is the mint guard of the tenant switch: an account a demo
// access signed in stays in its demo school. It reads inside the switch's
// own administrative transaction, so it holds for every caller and cannot be
// skipped by a route. Outside the demo environment no demo access exists.
func (s *AccountAuthentication) demoTenantLock(ctx context.Context, account domain.LoginAccount) error {
	demo, err := s.store.DemoAccountExists(ctx, account.ID)
	if err != nil {
		return failed("switch tenant", err)
	}
	if demo {
		return failed("switch tenant", domain.ErrDemoSessionTenantLocked)
	}
	return nil
}
