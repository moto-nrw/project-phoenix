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
// The team hears of a first access and of a changed contact consent. An
// address whose school did not fail gets no second school (#3463): the new
// link leads into the school it already has.
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
		var txErr error
		if access.SchoolSlug, txErr = d.schoolFor(txCtx, access, known, found); txErr != nil {
			return txErr
		}
		stored, lead = true, !found || known.ContactOptIn != access.ContactOptIn
		access.ID, txErr = d.store.InsertDemoAccess(txCtx, access)
		return txErr
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

// schoolFor returns the school a new access enters: the school of the
// address's newest unexpired access unless that school failed, otherwise a
// new one.
func (d *DemoAccess) schoolFor(ctx context.Context, access, known domain.DemoAccess, found bool) (string, error) {
	if found {
		entry, err := d.schools.DemoSchoolEntry(ctx, known.SchoolSlug)
		if err != nil {
			return "", err
		}
		if entry.Status != domain.DemoSchoolFailed {
			return known.SchoolSlug, nil
		}
	}
	return d.schools.PrepareDemoSchool(ctx, access.SchoolName, access.PersonName)
}

// Status reports the progress of the token's demo school and the access
// itself, whose school slug is the school's address once it is ready. A ready
// school without an account to sign in is still preparing.
func (d *DemoAccess) Status(ctx context.Context, token string) (access domain.DemoAccess, status string, err error) {
	err = d.adminTx(ctx, func(txCtx context.Context) error {
		var err error
		if access, err = d.valid(txCtx, token); err != nil {
			return err
		}
		entry, err := d.entry(txCtx, access.SchoolSlug)
		status = entry.Status
		return err
	})
	return access, status, err
}

// Redeem notes the use, which starts or resumes the school's simulation
// (#3464), and mints a session in the token's demo school: for
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
		if err := d.schools.MarkDemoSchoolUsed(txCtx, access.SchoolSlug, d.now()); err != nil {
			return err
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
