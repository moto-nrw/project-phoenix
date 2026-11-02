package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// TraegerAccounts is the Träger-Büro account management (ADR 0084 section 4,
// #4650). It owns the unit of work of every change: the invitation, the role
// set and the Träger sessions are its own rows, the Träger membership of
// Organisation & Tenancy joins the same administrative transaction as a
// participating command (ADR 0067), and the ledger entry of the Audit
// Platform commits with the change or the change rolls back.
//
// Every command re-checks the acting person against the database: the
// Träger must be live and the actor an active member with a switched-on
// account whose roles still grant "Konten verwalten". The rights in the
// token may be up to one access token lifetime old.
type TraegerAccounts struct {
	store       ports.TraegerAccountStore
	memberships identityPorts.TraegerBueroMemberships
	delivery    identityPorts.TraegerInvitationDelivery
	audit       identityPorts.TraegerAccountAudit
	passwords   identityPorts.PasswordPolicy
	// credentials checks the password of an existing account with the
	// login's attempt counting and lockout, in a unit of its own.
	credentials func(ctx context.Context, email, password, ipAddress, userAgent string) (int64, error)
	// runAdmin opens the administrative unit, or joins an ambient one.
	runAdmin func(context.Context, func(context.Context) error) error
	// inAdminUnit reports an ambient administrative transaction, the unit
	// the operator's invitation participates in.
	inAdminUnit func(context.Context) bool
	expiry      time.Duration
	now         func() time.Time
	logger      *slog.Logger
}

// TraegerAccountsDependencies are the ports the account management consumes.
type TraegerAccountsDependencies struct {
	Store       ports.TraegerAccountStore
	Memberships identityPorts.TraegerBueroMemberships
	Delivery    identityPorts.TraegerInvitationDelivery
	Audit       identityPorts.TraegerAccountAudit
	Passwords   identityPorts.PasswordPolicy
	// Credentials is the login's protected password check
	// (AccountAuthentication.CheckPasswordLogin).
	Credentials func(ctx context.Context, email, password, ipAddress, userAgent string) (int64, error)
	// Unit is the administrative unit New binds for the global identity
	// operations (newCommandTransactions().RunAdmin): it joins an ambient
	// transaction or opens one.
	Unit func(context.Context, func(context.Context) error) error
	// InAdminUnit reports an ambient administrative transaction.
	InAdminUnit func(context.Context) bool
	Expiry      time.Duration
	Logger      *slog.Logger
}

// NewTraegerAccounts composes the account management.
func NewTraegerAccounts(deps TraegerAccountsDependencies) (*TraegerAccounts, error) {
	if deps.Store == nil || deps.Memberships == nil || deps.Delivery == nil || deps.Audit == nil ||
		deps.Passwords == nil || deps.Credentials == nil || deps.Unit == nil || deps.InAdminUnit == nil {
		return nil, errors.New("identity access: every Träger account dependency is required")
	}
	if deps.Expiry <= 0 {
		return nil, errors.New("identity access: the Träger invitation lifetime is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &TraegerAccounts{
		store: deps.Store, memberships: deps.Memberships, delivery: deps.Delivery, audit: deps.Audit,
		passwords: deps.Passwords, credentials: deps.Credentials, runAdmin: deps.Unit, inAdminUnit: deps.InAdminUnit,
		expiry: deps.Expiry, now: time.Now, logger: logger.With("flow", "traeger_accounts"),
	}, nil
}

// traegerUnit is what every command of an acting person knows after the
// re-check: the Träger name, the active memberships and the Verwaltung role
// holders it locked.
type traegerUnit struct {
	name           string
	members        []identityPorts.TraegerMembership
	administrators []int64
}

func (u traegerUnit) member(accountID int64) (identityPorts.TraegerMembership, bool) {
	index := slices.IndexFunc(u.members, func(member identityPorts.TraegerMembership) bool { return member.AccountID == accountID })
	if index < 0 {
		return identityPorts.TraegerMembership{}, false
	}
	return u.members[index], true
}

func (u traegerUnit) memberIDs() []int64 {
	ids := make([]int64, 0, len(u.members))
	for _, member := range u.members {
		ids = append(ids, member.AccountID)
	}
	return ids
}

// activeAdministrators counts the locked Verwaltung holders that are still
// active members.
func (u traegerUnit) activeAdministrators() int {
	count := 0
	for _, accountID := range u.administrators {
		if _, ok := u.member(accountID); ok {
			count++
		}
	}
	return count
}

// run opens the administrative unit of an acting person and re-checks it.
func (t *TraegerAccounts) run(ctx context.Context, actor identityaccess.TraegerActor, fn func(context.Context, traegerUnit) error) error {
	if t == nil {
		return identityaccess.ErrTraegerAccountsUnavailable
	}
	if actor.OrganizationID <= 0 || actor.AccountID <= 0 {
		return identityaccess.ErrNoTraegerMembership
	}
	return t.runAdmin(ctx, func(txCtx context.Context) error {
		unit, err := t.begin(txCtx, actor)
		if err != nil {
			return err
		}
		return fn(txCtx, unit)
	})
}

func (t *TraegerAccounts) begin(ctx context.Context, actor identityaccess.TraegerActor) (traegerUnit, error) {
	administrators, _, err := t.store.LockTraegerAdministrators(ctx, actor.OrganizationID)
	if err != nil {
		return traegerUnit{}, err
	}
	traeger, live, err := t.memberships.FindLiveTraeger(ctx, actor.OrganizationID)
	if err != nil {
		return traegerUnit{}, err
	}
	if !live {
		return traegerUnit{}, identityaccess.ErrNoTraegerMembership
	}
	members, err := t.memberships.ListActiveTraegerMemberships(ctx, actor.OrganizationID)
	if err != nil {
		return traegerUnit{}, err
	}
	unit := traegerUnit{name: traeger.Name, members: members, administrators: administrators}
	if _, ok := unit.member(actor.AccountID); !ok {
		return traegerUnit{}, identityaccess.ErrNoTraegerMembership
	}
	// The token may outlive a switch-off of the account; the account itself
	// decides.
	account, found, _, err := t.store.FindLoginAccount(ctx, actor.AccountID, false)
	if err != nil {
		return traegerUnit{}, err
	}
	if !found || !account.Active {
		return traegerUnit{}, identityaccess.ErrAccountInactive
	}
	roles, _, err := t.store.ListTraegerRoles(ctx, actor.OrganizationID, actor.AccountID)
	if err != nil {
		return traegerUnit{}, err
	}
	if !slices.Contains(identityaccess.TraegerRights(roles), identityaccess.TraegerRightAccountsManage) {
		return traegerUnit{}, identityaccess.ErrTraegerAccountsForbidden
	}
	return unit, nil
}

// ListTraegerBuero returns the active people and the open invitations. The
// read names colleagues' addresses and is written to the ledger.
func (t *TraegerAccounts) ListTraegerBuero(ctx context.Context, actor identityaccess.TraegerActor) (identityaccess.TraegerBuero, error) {
	result := identityaccess.TraegerBuero{Members: []identityaccess.TraegerMember{}, Invitations: []identityaccess.TraegerInvitation{}}
	err := t.run(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		accounts, _, err := t.store.ListTraegerAccounts(txCtx, actor.OrganizationID, unit.memberIDs())
		if err != nil {
			return err
		}
		for _, account := range accounts {
			member, _ := unit.member(account.AccountID)
			result.Members = append(result.Members, publicTraegerMember(account, member))
		}
		invitations, _, err := t.store.ListOpenTraegerInvitations(txCtx, actor.OrganizationID, t.now())
		if err != nil {
			return err
		}
		for _, invitation := range invitations {
			result.Invitations = append(result.Invitations, publicTraegerInvitation(invitation))
		}
		return t.record(txCtx, actor, identityaccess.TraegerAccountMembersListed, nil, nil, nil, nil)
	})
	return result, err
}

// InviteTraegerAccount spends any open invitation of the address, stores a
// new one and queues its mail in the same unit.
func (t *TraegerAccounts) InviteTraegerAccount(ctx context.Context, actor identityaccess.TraegerActor, email string, roles []string) (identityaccess.TraegerInvitation, error) {
	email, roles, err := normalizeTraegerInvitation(email, roles)
	if err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	var created domain.TraegerInvitation
	err = t.run(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		created, err = t.invite(txCtx, actor, unit, email, roles)
		return err
	})
	if err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	t.logger.Info("traeger_invitation_created",
		slog.Int64("organization_id", actor.OrganizationID),
		slog.Int64("invitation_id", created.ID),
		slog.Int64("actor_account_id", actor.AccountID))
	invitation := publicTraegerInvitation(created)
	invitation.Token = ""
	return invitation, nil
}

// invite stores and mails the invitation of an acting person and writes it
// to the ledger.
func (t *TraegerAccounts) invite(ctx context.Context, actor identityaccess.TraegerActor, unit traegerUnit, email string, roles []string) (domain.TraegerInvitation, error) {
	createdBy := actor.AccountID
	created, err := t.storeInvitation(ctx, actor.OrganizationID, &createdBy, unit, email, roles)
	if err != nil {
		return domain.TraegerInvitation{}, err
	}
	return created, t.record(ctx, actor, identityaccess.TraegerAccountInvitationCreated, nil, &email, nil, roles)
}

// InviteTraegerAccountForOperator stores and mails an invitation the moto
// team sends from the operator portal (#5144), on the caller's
// administrative transaction. The invitation has no inviting account; the
// operator audit of the caller names the operator. The ledger of the
// Träger-Büro records the acceptance as usual.
func (t *TraegerAccounts) InviteTraegerAccountForOperator(ctx context.Context, organizationID int64, email string, roles []string) (identityaccess.TraegerInvitation, error) {
	if t == nil {
		return identityaccess.TraegerInvitation{}, identityaccess.ErrTraegerAccountsUnavailable
	}
	if !t.inAdminUnit(ctx) {
		return identityaccess.TraegerInvitation{}, identityaccess.ErrTraegerTransactionRequired
	}
	if organizationID <= 0 {
		return identityaccess.TraegerInvitation{}, identityaccess.ErrNoTraegerMembership
	}
	email, roles, err := normalizeTraegerInvitation(email, roles)
	if err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	// The Verwaltung lock first, like every command of the Träger-Büro: an
	// invitation of the same address from the Träger-Portal waits.
	if _, _, err := t.store.LockTraegerAdministrators(ctx, organizationID); err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	traeger, live, err := t.memberships.FindLiveTraeger(ctx, organizationID)
	if err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	if !live {
		return identityaccess.TraegerInvitation{}, identityaccess.ErrNoTraegerMembership
	}
	members, err := t.memberships.ListActiveTraegerMemberships(ctx, organizationID)
	if err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	created, err := t.storeInvitation(ctx, organizationID, nil, traegerUnit{name: traeger.Name, members: members}, email, roles)
	if err != nil {
		return identityaccess.TraegerInvitation{}, err
	}
	invitation := publicTraegerInvitation(created)
	invitation.Token = ""
	return invitation, nil
}

// storeInvitation refuses an address that already belongs to the
// Träger-Büro, then replaces its open invitation and queues the mail.
func (t *TraegerAccounts) storeInvitation(ctx context.Context, organizationID int64, createdBy *int64, unit traegerUnit, email string, roles []string) (domain.TraegerInvitation, error) {
	account, found, _, err := t.store.FindLoginAccountByEmail(ctx, email)
	if err != nil {
		return domain.TraegerInvitation{}, err
	}
	if _, member := unit.member(account.ID); found && member {
		return domain.TraegerInvitation{}, identityaccess.ErrTraegerAlreadyMember
	}
	now := t.now()
	if _, err := t.store.RevokeOpenTraegerInvitationsForEmail(ctx, organizationID, email, now); err != nil {
		return domain.TraegerInvitation{}, err
	}
	created, _, err := t.store.InsertTraegerInvitation(ctx, domain.TraegerInvitation{
		OrganizationID: organizationID, Email: email, Token: uuid.Must(uuid.NewV4()).String(),
		Roles: roles, ExpiresAt: now.Add(t.expiry), CreatedBy: createdBy,
	})
	if err != nil {
		return domain.TraegerInvitation{}, err
	}
	if err := t.delivery.DispatchTraegerInvitation(ctx, publicTraegerInvitation(created), unit.name, t.expiry); err != nil {
		return domain.TraegerInvitation{}, fmt.Errorf("queue Träger invitation mail: %w", err)
	}
	return created, nil
}

// RevokeTraegerInvitation spends an open invitation of the Träger.
func (t *TraegerAccounts) RevokeTraegerInvitation(ctx context.Context, actor identityaccess.TraegerActor, invitationID int64) error {
	return t.run(ctx, actor, func(txCtx context.Context, _ traegerUnit) error {
		invitation, found, _, err := t.store.RevokeTraegerInvitation(txCtx, actor.OrganizationID, invitationID, t.now())
		if err != nil {
			return err
		}
		if !found {
			return identityaccess.ErrInvitationNotFound
		}
		t.logger.Info("traeger_invitation_revoked",
			slog.Int64("organization_id", actor.OrganizationID),
			slog.Int64("invitation_id", invitation.ID),
			slog.Int64("actor_account_id", actor.AccountID))
		return t.record(txCtx, actor, identityaccess.TraegerAccountInvitationRevoked, nil, &invitation.Email, invitation.Roles, nil)
	})
}

// ChangeTraegerRoles replaces the role set of an active member. The session
// of that person picks the new rights up at its next refresh.
func (t *TraegerAccounts) ChangeTraegerRoles(ctx context.Context, actor identityaccess.TraegerActor, accountID int64, roles []string) (identityaccess.TraegerMember, error) {
	roles, err := normalizeTraegerRoles(roles)
	if err != nil {
		return identityaccess.TraegerMember{}, err
	}
	var result identityaccess.TraegerMember
	err = t.run(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		membership, ok := unit.member(accountID)
		if !ok {
			return identityaccess.ErrTraegerMemberNotFound
		}
		before, _, err := t.store.ListTraegerRoles(txCtx, actor.OrganizationID, accountID)
		if err != nil {
			return err
		}
		if leavesNoAdministrator(unit, accountID, before, roles) {
			return identityaccess.ErrTraegerLastAdministrator
		}
		if _, err := t.store.ReplaceTraegerRoles(txCtx, actor.OrganizationID, accountID, roles); err != nil {
			return err
		}
		accounts, _, err := t.store.ListTraegerAccounts(txCtx, actor.OrganizationID, []int64{accountID})
		if err != nil {
			return err
		}
		if len(accounts) != 1 {
			return identityaccess.ErrTraegerMemberNotFound
		}
		result = publicTraegerMember(accounts[0], membership)
		return t.record(txCtx, actor, identityaccess.TraegerAccountRolesChanged, &accountID, &accounts[0].Email, before, roles)
	})
	return result, err
}

// RemoveTraegerMember ends the membership of an active member, withdraws its
// roles and ends its Träger sessions at once.
func (t *TraegerAccounts) RemoveTraegerMember(ctx context.Context, actor identityaccess.TraegerActor, accountID int64) error {
	return t.run(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		if _, ok := unit.member(accountID); !ok {
			return identityaccess.ErrTraegerMemberNotFound
		}
		accounts, _, err := t.store.ListTraegerAccounts(txCtx, actor.OrganizationID, []int64{accountID})
		if err != nil {
			return err
		}
		if len(accounts) != 1 {
			return identityaccess.ErrTraegerMemberNotFound
		}
		before := accounts[0].Roles
		if leavesNoAdministrator(unit, accountID, before, nil) {
			return identityaccess.ErrTraegerLastAdministrator
		}
		if _, err := t.store.ReplaceTraegerRoles(txCtx, actor.OrganizationID, accountID, nil); err != nil {
			return err
		}
		if err := t.memberships.DeactivateTraegerMembership(txCtx, actor.OrganizationID, accountID); err != nil {
			return err
		}
		if _, err := t.store.DeleteTraegerAccountSessions(txCtx, actor.OrganizationID, accountID); err != nil {
			return err
		}
		return t.record(txCtx, actor, identityaccess.TraegerAccountMemberRemoved, &accountID, &accounts[0].Email, before, nil)
	})
}

// PreviewTraegerInvitation tells the holder of a redeemable link where it
// leads and whether the address already has an account.
func (t *TraegerAccounts) PreviewTraegerInvitation(ctx context.Context, token string) (identityaccess.TraegerInvitationPreview, error) {
	if t == nil {
		return identityaccess.TraegerInvitationPreview{}, identityaccess.ErrTraegerAccountsUnavailable
	}
	var preview identityaccess.TraegerInvitationPreview
	err := t.runAdmin(ctx, func(txCtx context.Context) error {
		invitation, traeger, err := t.redeemable(txCtx, token, false)
		if err != nil {
			return err
		}
		_, found, _, err := t.store.FindLoginAccountByEmail(txCtx, invitation.Email)
		if err != nil {
			return err
		}
		preview = identityaccess.TraegerInvitationPreview{
			OrganizationName: traeger.Name, Email: invitation.Email, Roles: invitation.Roles,
			RequiresExistingPassword: found, ExpiresAt: invitation.ExpiresAt,
		}
		return nil
	})
	return preview, err
}

// AcceptTraegerInvitation creates the account or proves the existing one,
// grants the invited roles, activates the membership and spends the link,
// all in one unit. An existing account proves itself first, through the
// login's password check with its attempt counting and lockout; that check
// commits in its own unit, so a refused guess stays counted.
func (t *TraegerAccounts) AcceptTraegerInvitation(ctx context.Context, token string, acceptance identityaccess.TraegerInvitationAcceptance) (string, error) {
	if t == nil {
		return "", identityaccess.ErrTraegerAccountsUnavailable
	}
	verified, err := t.proveExistingAccount(ctx, token, acceptance)
	if err != nil {
		return "", err
	}
	var actor identityaccess.TraegerActor
	var email string
	err = t.runAdmin(ctx, func(txCtx context.Context) error {
		var err error
		actor, email, err = t.accept(txCtx, token, verified, acceptance)
		return err
	})
	if err != nil {
		return "", err
	}
	t.logger.Info("traeger_invitation_accepted",
		slog.Int64("organization_id", actor.OrganizationID),
		slog.Int64("account_id", actor.AccountID))
	return email, nil
}

// proveExistingAccount returns the account behind the invited address after
// the protected password check, or zero for an address without an account.
func (t *TraegerAccounts) proveExistingAccount(ctx context.Context, token string, acceptance identityaccess.TraegerInvitationAcceptance) (int64, error) {
	var email string
	var exists bool
	err := t.runAdmin(ctx, func(txCtx context.Context) error {
		invitation, _, err := t.redeemable(txCtx, token, false)
		if err != nil {
			return err
		}
		email = invitation.Email
		_, exists, _, err = t.store.FindLoginAccountByEmail(txCtx, email)
		return err
	})
	if err != nil || !exists {
		return 0, err
	}
	return t.credentials(ctx, email, acceptance.Password, acceptance.IPAddress, acceptance.UserAgent)
}

// accept runs the acceptance on the administrative unit and returns the new
// member and its address.
func (t *TraegerAccounts) accept(ctx context.Context, token string, verified int64, acceptance identityaccess.TraegerInvitationAcceptance) (identityaccess.TraegerActor, string, error) {
	invitation, _, err := t.redeemable(ctx, token, true)
	if err != nil {
		return identityaccess.TraegerActor{}, "", err
	}
	account, err := t.invitedAccount(ctx, invitation.Email, verified, acceptance)
	if err != nil {
		return identityaccess.TraegerActor{}, "", err
	}
	actor := identityaccess.TraegerActor{OrganizationID: invitation.OrganizationID, AccountID: account.ID}
	before, _, err := t.store.ListTraegerRoles(ctx, invitation.OrganizationID, account.ID)
	if err != nil {
		return actor, "", err
	}
	granted := unionTraegerRoles(before, invitation.Roles)
	// The account row is locked first, then the role rows, then the
	// membership: the order the operator's grant takes too.
	if _, err := t.store.ReplaceTraegerRoles(ctx, invitation.OrganizationID, account.ID, granted); err != nil {
		return actor, "", err
	}
	if err := t.memberships.ActivateTraegerMembership(ctx, invitation.OrganizationID, account.ID); err != nil {
		return actor, "", err
	}
	spent, _, err := t.store.SpendTraegerInvitation(ctx, invitation.ID, t.now())
	if err != nil {
		return actor, "", err
	}
	if !spent {
		return actor, "", identityaccess.ErrInvitationUsed
	}
	return actor, account.Email, t.record(ctx, actor, identityaccess.TraegerAccountInvitationAccepted, &account.ID, &account.Email, before, granted)
}

// redeemable reads the invitation behind the token and refuses an unknown,
// spent or expired link and one of a Träger that is no longer live.
func (t *TraegerAccounts) redeemable(ctx context.Context, token string, forUpdate bool) (domain.TraegerInvitation, identityaccess.TraegerOrganization, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, identityaccess.ErrInvitationNotFound
	}
	invitation, found, _, err := t.store.FindTraegerInvitationByToken(ctx, token, forUpdate)
	if err != nil {
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, err
	}
	switch {
	case !found:
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, identityaccess.ErrInvitationNotFound
	case !invitation.Open():
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, identityaccess.ErrInvitationUsed
	case !invitation.ExpiresAt.After(t.now()):
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, identityaccess.ErrInvitationExpired
	}
	traeger, live, err := t.memberships.FindLiveTraeger(ctx, invitation.OrganizationID)
	if err != nil {
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, err
	}
	if !live {
		return domain.TraegerInvitation{}, identityaccess.TraegerOrganization{}, identityaccess.ErrInvitationNotFound
	}
	return invitation, traeger, nil
}

// invitedAccount takes the existing account the password check proved, or
// creates the account with the password chosen twice. The address row lock
// serializes with account retention; an account that appeared or vanished
// since the check is refused like a wrong password.
func (t *TraegerAccounts) invitedAccount(ctx context.Context, email string, verified int64, acceptance identityaccess.TraegerInvitationAcceptance) (domain.LoginAccount, error) {
	account, found, _, err := t.store.LockLoginAccountByEmail(ctx, email)
	if err != nil {
		return domain.LoginAccount{}, err
	}
	if found || verified != 0 {
		switch {
		case !found || account.ID != verified:
			return domain.LoginAccount{}, identityaccess.ErrInvalidCredentials
		case !account.Active:
			return domain.LoginAccount{}, identityaccess.ErrAccountInactive
		}
		return account, nil
	}
	if acceptance.Password != acceptance.ConfirmPassword {
		return domain.LoginAccount{}, identityaccess.ErrInvitationPasswordMismatch
	}
	if err := t.passwords.ValidatePasswordStrength(acceptance.Password); err != nil {
		return domain.LoginAccount{}, err
	}
	hash, err := t.passwords.HashPassword(acceptance.Password)
	if err != nil {
		return domain.LoginAccount{}, err
	}
	created, _, err := t.store.InsertAccount(ctx, email, hash)
	return created, err
}

func (t *TraegerAccounts) record(ctx context.Context, actor identityaccess.TraegerActor, action string, targetAccountID *int64, targetEmail *string, before, after []string) error {
	return t.audit.RecordTraegerAccountChange(ctx, identityaccess.TraegerAccountChange{
		OrganizationID: actor.OrganizationID, ActorAccountID: actor.AccountID, Action: action,
		TargetAccountID: targetAccountID, TargetEmail: targetEmail, RolesBefore: before, RolesAfter: after,
	})
}

// normalizeTraegerInvitation lowercases the address and normalizes the role
// set of an invitation.
func normalizeTraegerInvitation(email string, roles []string) (string, []string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return "", nil, identityaccess.ErrTraegerInvalidEmail
	}
	roles, err := normalizeTraegerRoles(roles)
	return email, roles, err
}

// normalizeTraegerRoles deduplicates and sorts a role set and refuses an
// empty set or an unknown role.
func normalizeTraegerRoles(roles []string) ([]string, error) {
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if !identityaccess.IsTraegerRole(role) {
			return nil, identityaccess.ErrInvalidTraegerRole
		}
		if !slices.Contains(result, role) {
			result = append(result, role)
		}
	}
	if len(result) == 0 {
		return nil, identityaccess.ErrInvalidTraegerRole
	}
	slices.Sort(result)
	return result, nil
}

func unionTraegerRoles(held, granted []string) []string {
	result := slices.Clone(held)
	for _, role := range granted {
		if !slices.Contains(result, role) {
			result = append(result, role)
		}
	}
	slices.Sort(result)
	return result
}

// leavesNoAdministrator reports a change of the account from before to
// after that takes the role Verwaltung from the last usable person holding
// it. Only usable holders count (LockTraegerAdministrators): a switched-off
// account cannot sign in, so it neither keeps the Träger-Büro manageable nor
// blocks its own removal.
func leavesNoAdministrator(unit traegerUnit, accountID int64, before, after []string) bool {
	losesAdministration := slices.Contains(before, identityaccess.TraegerRoleAdministration) &&
		!slices.Contains(after, identityaccess.TraegerRoleAdministration) &&
		slices.Contains(unit.administrators, accountID)
	return losesAdministration && unit.activeAdministrators() <= 1
}

func publicTraegerMember(account domain.TraegerAccount, membership identityPorts.TraegerMembership) identityaccess.TraegerMember {
	roles := account.Roles
	if roles == nil {
		roles = []string{}
	}
	return identityaccess.TraegerMember{AccountID: account.AccountID, Email: account.Email, Roles: roles, ActivatedAt: membership.ActivatedAt}
}

func publicTraegerInvitation(invitation domain.TraegerInvitation) identityaccess.TraegerInvitation {
	return identityaccess.TraegerInvitation{
		ID: invitation.ID, OrganizationID: invitation.OrganizationID, Email: invitation.Email, Roles: invitation.Roles,
		Token: invitation.Token, ExpiresAt: invitation.ExpiresAt, CreatedAt: invitation.CreatedAt,
	}
}
