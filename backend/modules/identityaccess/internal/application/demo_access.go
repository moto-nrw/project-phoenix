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
	mail     ports.DemoAccessMail
	adminTx  ports.DemoAdminTx
	now      func() time.Time
}

// DemoAccessDependencies are the ports the flows consume.
type DemoAccessDependencies struct {
	Sessions ports.DemoSessions
	Store    ports.DemoAccessStore
	Schools  ports.DemoSchools
	Tokens   ports.DemoAccessTokens
	Mail     ports.DemoAccessMail
	AdminTx  ports.DemoAdminTx
}

func NewDemoAccess(deps DemoAccessDependencies) (*DemoAccess, error) {
	if deps.Sessions == nil || deps.Store == nil || deps.Schools == nil || deps.Tokens == nil || deps.Mail == nil || deps.AdminTx == nil {
		return nil, fmt.Errorf("identity access demo access: every dependency is required")
	}
	return &DemoAccess{
		sessions: deps.Sessions, store: deps.Store, schools: deps.Schools, tokens: deps.Tokens, mail: deps.Mail, adminTx: deps.AdminTx, now: time.Now,
	}, nil
}

// Request stores a demo access and mails its link (#3465). The link leaves
// by mail only, so nobody enters a demo with somebody else's address and the
// answer tells nothing about the address. Every request stores its own
// access: earlier links stay valid until they expire, and the prospect's
// latest details are kept. An address inside its cooldown gets nothing new.
// The team hears of a first access and of a changed contact consent.
func (d *DemoAccess) Request(ctx context.Context, access domain.DemoAccess, entryURLPrefix string) error {
	if err := access.Normalize(); err != nil {
		return err
	}
	raw, fingerprint, err := d.tokens.NewToken()
	if err != nil {
		return fmt.Errorf("mint demo access token: %w", err)
	}
	access.TokenHash = fingerprint
	access.ExpiresAt = d.now().Add(domain.DemoAccessLifetime)
	var stored, lead bool
	err = d.adminTx(ctx, func(txCtx context.Context) error {
		known, found, findErr := d.store.FindActiveDemoAccessByEmail(txCtx, access.Email, d.now())
		if findErr != nil || (found && known.CoolingDown(d.now())) {
			return findErr
		}
		stored, lead = true, !found || known.ContactOptIn != access.ContactOptIn
		var insertErr error
		access.ID, insertErr = d.store.InsertDemoAccess(txCtx, access)
		return insertErr
	})
	if err != nil {
		return fmt.Errorf("store demo access: %w", err)
	}
	if !stored {
		return nil
	}
	d.mail.SendDemoAccessLink(ctx, access, entryURLPrefix+raw)
	if lead {
		d.mail.SendDemoLead(ctx, access)
	}
	return nil
}

// Ready reports whether the token's demo school can be entered.
func (d *DemoAccess) Ready(ctx context.Context, token, schoolSlug string) (bool, error) {
	var ready bool
	err := d.adminTx(ctx, func(txCtx context.Context) error {
		if _, err := d.valid(txCtx, token); err != nil {
			return err
		}
		_, _, found, err := d.administrator(txCtx, schoolSlug)
		ready = found
		return err
	})
	return ready, err
}

// Redeem notes the use and mints a session for the demo school's
// administrator. The token stays valid afterwards.
func (d *DemoAccess) Redeem(ctx context.Context, token, schoolSlug, ipAddress, userAgent string) (string, string, error) {
	var accountID, tenantID int64
	err := d.adminTx(ctx, func(txCtx context.Context) error {
		access, err := d.valid(txCtx, token)
		if err != nil {
			return err
		}
		var found bool
		accountID, tenantID, found, err = d.administrator(txCtx, schoolSlug)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrDemoSchoolPreparing
		}
		return d.store.RecordDemoAccessUse(txCtx, access.ID, accountID, d.now())
	})
	if err != nil {
		return "", "", err
	}
	return d.sessions.IssueTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
}

// administrator is the account a demo session signs in: the school exists
// and has an administrator once the demo process provisioned it.
func (d *DemoAccess) administrator(ctx context.Context, schoolSlug string) (accountID, tenantID int64, found bool, err error) {
	tenantID, found, err = d.schools.FindDemoSchool(ctx, schoolSlug)
	if err != nil || !found {
		return 0, 0, false, err
	}
	accountID, found, err = d.store.FindSchoolAdministrator(ctx, tenantID)
	return accountID, tenantID, found, err
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
