package application

import (
	"context"
	"errors"
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
	// perIP and perAddress limit the public request (#3466).
	perIP      *demoRequestWindow
	perAddress *demoRequestWindow
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
		perIP: newDemoRequestWindow(domain.DemoRequestsPerIPAddress), perAddress: newDemoRequestWindow(domain.DemoRequestsPerAddress),
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
//
// Only a valid request counts against its IP address and its address, so
// typing errors of fair visitors behind one WLAN address fill no window; an
// invalid request touches no table (#3466). A new school beyond the
// configured number is refused by PrepareDemoSchool; such a request gives
// its places in both windows back, so retries at the capacity do not end in
// a rate limit once a place is free again.
func (d *DemoAccess) Request(ctx context.Context, access domain.DemoAccess, clientIP, entryURLPrefix string) error {
	if err := access.Normalize(); err != nil {
		return err
	}
	at := d.now()
	if err := d.perIP.admit(clientIP, at); err != nil {
		return err
	}
	if err := d.perAddress.admit(access.Email, at); err != nil {
		d.perIP.withdraw(clientIP, at)
		return err
	}
	err := d.request(ctx, access, entryURLPrefix)
	if errors.Is(err, domain.ErrDemoCapacityReached) {
		d.perIP.withdraw(clientIP, at)
		d.perAddress.withdraw(access.Email, at)
	}
	return err
}

func (d *DemoAccess) request(ctx context.Context, access domain.DemoAccess, entryURLPrefix string) error {
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
	entryURL := entryURLPrefix + raw
	if access.Role != "" {
		// The fragment carries the preselected role past the waiting room.
		entryURL += "&role=" + string(access.Role)
	}
	d.mail.SendDemoAccessLink(ctx, access, entryURL)
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
//
// A chosen demo role (#3467) first becomes the role of the prospect's own
// caregiver, so a switch in the banner is a further redemption: one account,
// a new session. The shared administrator of the standing school keeps its
// role; its session has all functions whatever was chosen.
//
// The demo role parent (#3468) leaves the caregiver's role alone and mints a
// parents portal session for the school's parent who carries the visitor's
// name, on the parents host with the same token. The standing school has no
// such parent; its session stays the administrator's.
func (d *DemoAccess) Redeem(ctx context.Context, token string, role domain.DemoRole, ipAddress, userAgent string) (domain.DemoEntry, error) {
	role, err := domain.ParseDemoRole(string(role))
	if err != nil {
		return domain.DemoEntry{}, err
	}
	var entry domain.DemoSchoolEntry
	var access domain.DemoAccess
	err = d.adminTx(ctx, func(txCtx context.Context) error {
		var err error
		if access, err = d.valid(txCtx, token); err != nil {
			return err
		}
		if entry, err = d.entry(txCtx, access.SchoolSlug); err != nil {
			return err
		}
		if entry.Status != domain.DemoSchoolReady {
			return domain.ErrDemoSchoolPreparing
		}
		if role, err = d.assumeRole(txCtx, entry, role); err != nil {
			return err
		}
		if err := d.schools.MarkDemoSchoolUsed(txCtx, access.SchoolSlug, d.now()); err != nil {
			return err
		}
		// The access keeps naming the caregiver, also for the role parent;
		// the parent it signs in is noted next to it, so the tenant lock and
		// the session cap of the demo hold for that account too (#3462).
		return d.store.RecordDemoAccessUse(txCtx, access.ID, entry.AccountID, signedInParent(entry, role), d.now())
	})
	if err != nil {
		return domain.DemoEntry{}, err
	}
	var accessToken, refreshToken string
	if role == domain.DemoRoleParent {
		accessToken, refreshToken, err = d.sessions.IssueParentTokensForAuthenticatedAccount(ctx, entry.ParentAccountID, ipAddress, userAgent)
	} else {
		accessToken, refreshToken, err = d.sessions.IssueTokensForAuthenticatedAccount(ctx, entry.AccountID, entry.TenantID, ipAddress, userAgent)
	}
	if err != nil {
		return domain.DemoEntry{}, err
	}
	return domain.DemoEntry{
		AccessToken: accessToken, RefreshToken: refreshToken,
		AccessID: access.ID, Role: role, Source: access.Source, FixedRole: entry.Shared,
	}, nil
}

// signedInParent names the account a redemption signs in besides the
// caregiver: the school's visitor parent, and only for the role parent.
func signedInParent(entry domain.DemoSchoolEntry, role domain.DemoRole) int64 {
	if role != domain.DemoRoleParent {
		return 0
	}
	return entry.ParentAccountID
}

// assumeRole gives the prospect's own caregiver the chosen demo role and
// returns the role the session will have. The shared administrator keeps
// all functions; no role keeps the account as it is. The parent role needs
// the school's visitor parent and changes no staff role.
func (d *DemoAccess) assumeRole(ctx context.Context, entry domain.DemoSchoolEntry, role domain.DemoRole) (domain.DemoRole, error) {
	if entry.Shared {
		return domain.DemoRoleAll, nil
	}
	if role == "" {
		return role, nil
	}
	if role == domain.DemoRoleParent {
		if entry.ParentAccountID == 0 {
			return "", domain.ErrDemoAccessInvalid
		}
		return role, nil
	}
	return role, d.store.ReplaceDemoAccountRole(ctx, entry.AccountID, entry.TenantID, role.SchoolRole())
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
	entry.AccountID, entry.Shared = accountID, true
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
