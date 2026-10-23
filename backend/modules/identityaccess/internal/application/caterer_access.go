package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// errSchoolTransactionRequired guards the caterer writes: they run in the
// transaction of the school they concern, which the caller opens.
var errSchoolTransactionRequired = errors.New("identity access: the caterer flows need the school's transaction")

// CatererAccess is the caterer's identity (ADR 0077): a school invites its
// caterer, the caterer accepts with a password, and signs in to the caterer
// portal. Nothing here opens a transaction of its own (#4088): writes join
// the school's transaction the caller opens, and the caterer of an account
// and its schools come from the global identity read. The caller, the
// caterer portal workflow, serializes the supply's lifecycle with its own
// lock; these commands take none.
type CatererAccess struct {
	auth      *AccountAuthentication
	store     ports.CatererStore
	delivery  ports.CatererInvitationDelivery
	passwords ports.PasswordHasher
	verifier  ports.PasswordVerifier
	schools   ports.SchoolDirectory
	runtime   ports.Runtime
	attach    func(context.Context) context.Context
	now       func() time.Time
	logger    *slog.Logger
}

// CatererAccessDependencies are the ports the caterer flows consume.
type CatererAccessDependencies struct {
	Store     ports.CatererStore
	Delivery  ports.CatererInvitationDelivery
	Passwords ports.PasswordHasher
	Verifier  ports.PasswordVerifier
	Schools   ports.SchoolDirectory
	Runtime   ports.Runtime
	// Attach binds the composed unit of work ahead of every flow; nil keeps
	// the context.
	Attach func(context.Context) context.Context
	Logger *slog.Logger
}

func NewCatererAccess(auth *AccountAuthentication, deps CatererAccessDependencies) (*CatererAccess, error) {
	if auth == nil || deps.Store == nil || deps.Delivery == nil || deps.Passwords == nil || deps.Verifier == nil || deps.Schools == nil || deps.Runtime == nil {
		return nil, fmt.Errorf("identity access caterer access: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	attach := deps.Attach
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return &CatererAccess{auth: auth, store: deps.Store, delivery: deps.Delivery, passwords: deps.Passwords, verifier: deps.Verifier, schools: deps.Schools, runtime: deps.Runtime, attach: attach, now: time.Now, logger: logger}, nil
}

// inSchool reports whether the caller opened a school's transaction.
func (c *CatererAccess) inSchool(ctx context.Context) bool {
	return c.runtime.HasTransaction(ctx) && c.runtime.TenantID(ctx) > 0
}

// CreateCatererInvitation issues the school's invitation and mails the link
// to the caterer. The link is never stored and never returned: only the
// owner of the address can accept. Earlier open invitations of the school
// are taken back; a school has one caterer.
func (c *CatererAccess) CreateCatererInvitation(ctx context.Context, input identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error) {
	ctx = c.attach(ctx)
	invitation, token, err := c.createInvitation(ctx, domain.CatererInvitationRequest(input))
	if err == nil {
		err = c.delivery.EnqueueCatererInvitation(ctx, invitation.ID, invitation.TenantID, invitation.Email, invitation.CatererName, token, invitation.ExpiresAt)
	}
	if err != nil {
		return identityaccess.CatererInvitation{}, catererError(failed("create caterer invitation", err))
	}
	c.logger.Info("caterer invitation created",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("created_by", input.CreatedBy))
	return publicCatererInvitation(invitation), nil
}

func (c *CatererAccess) createInvitation(ctx context.Context, request domain.CatererInvitationRequest) (domain.CatererInvitation, string, error) {
	if !c.inSchool(ctx) {
		return domain.CatererInvitation{}, "", errSchoolTransactionRequired
	}
	request, err := request.Normalized()
	if err != nil {
		return domain.CatererInvitation{}, "", err
	}
	token, err := newInvitationToken()
	if err != nil {
		return domain.CatererInvitation{}, "", err
	}
	if _, active, _, err := c.store.FindSchoolCaterer(ctx); err != nil {
		return domain.CatererInvitation{}, "", fmt.Errorf("check caterer supply: %w", err)
	} else if active {
		return domain.CatererInvitation{}, "", domain.ErrCatererAlreadySupplies
	}
	if err := c.revokeOpen(ctx); err != nil {
		return domain.CatererInvitation{}, "", err
	}
	createdBy := request.CreatedBy
	invitation, _, err := c.store.InsertCatererInvitation(ctx, domain.CatererInvitation{
		TenantID: c.runtime.TenantID(ctx), TokenHash: domain.HashInvitationToken(token), Email: request.Email,
		CatererName: request.CatererName, DeliveryAddress: request.DeliveryAddress, CreatedBy: &createdBy,
		ExpiresAt: c.now().Add(domain.CatererInvitationLifetime),
	})
	return invitation, token, err
}

// revokeOpen takes back the school's invitations still open.
func (c *CatererAccess) revokeOpen(ctx context.Context) error {
	invitations, _, err := c.store.ListCatererInvitations(ctx)
	if err != nil {
		return err
	}
	for _, invitation := range invitations {
		if invitation.Open(c.now()) {
			if _, _, err := c.store.RevokeCatererInvitation(ctx, invitation.ID, c.now()); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListCatererInvitations lists the school's invitations, newest first.
func (c *CatererAccess) ListCatererInvitations(ctx context.Context) ([]identityaccess.CatererInvitation, error) {
	ctx = c.attach(ctx)
	if !c.inSchool(ctx) {
		return nil, catererError(failed("list caterer invitations", errSchoolTransactionRequired))
	}
	invitations, _, err := c.store.ListCatererInvitations(ctx)
	if err != nil {
		return nil, catererError(failed("list caterer invitations", err))
	}
	result := make([]identityaccess.CatererInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, publicCatererInvitation(invitation))
	}
	return result, nil
}

// RevokeCatererInvitation takes back an open invitation of the school.
func (c *CatererAccess) RevokeCatererInvitation(ctx context.Context, invitationID int64) error {
	ctx = c.attach(ctx)
	if !c.inSchool(ctx) {
		return catererError(failed("revoke caterer invitation", errSchoolTransactionRequired))
	}
	revoked, _, err := c.store.RevokeCatererInvitation(ctx, invitationID, c.now())
	if err == nil && !revoked {
		err = domain.ErrInvitationNotFound
	}
	if err != nil {
		return catererError(failed("revoke caterer invitation", err))
	}
	return nil
}

// EndCatererSchool ends the school's supply by its caterer. The caterer
// loses the school at once.
func (c *CatererAccess) EndCatererSchool(ctx context.Context, accountID int64) (bool, error) {
	ctx = c.attach(ctx)
	if !c.inSchool(ctx) {
		return false, catererError(failed("end caterer school", errSchoolTransactionRequired))
	}
	ended, _, err := c.store.EndCatererSchool(ctx, accountID, c.now())
	if err != nil {
		return false, catererError(failed("end caterer school", err))
	}
	if ended {
		c.logger.Info("caterer supply ended",
			slog.Int64("ended_by", accountID),
			slog.Int64("tenant_id", c.runtime.TenantID(ctx)))
	}
	return ended, nil
}

// SchoolCaterer reads the school's running supply: the caterer, where it
// delivers and since when. The invitation that started it may long be
// deleted.
func (c *CatererAccess) SchoolCaterer(ctx context.Context) (identityaccess.CatererSupply, bool, error) {
	ctx = c.attach(ctx)
	if !c.inSchool(ctx) {
		return identityaccess.CatererSupply{}, false, catererError(failed("find school caterer", errSchoolTransactionRequired))
	}
	supply, found, _, err := c.store.FindSchoolCaterer(ctx)
	if err != nil {
		return identityaccess.CatererSupply{}, false, catererError(failed("find school caterer", err))
	}
	return identityaccess.CatererSupply{
		Caterer:         identityaccess.Caterer(supply.Caterer),
		DeliveryAddress: supply.DeliveryAddress, StartedAt: supply.StartedAt,
	}, found, nil
}

// PreviewCatererInvitation is what the public acceptance page shows. The
// link names the school, so the caller opens that school's transaction.
func (c *CatererAccess) PreviewCatererInvitation(ctx context.Context, token string) (identityaccess.CatererInvitationPreview, error) {
	preview, err := c.preview(c.attach(ctx), token)
	if err != nil {
		return identityaccess.CatererInvitationPreview{}, catererError(failed("preview caterer invitation", err))
	}
	return identityaccess.CatererInvitationPreview(preview), nil
}

func (c *CatererAccess) preview(ctx context.Context, token string) (domain.CatererInvitationPreview, error) {
	if !c.inSchool(ctx) {
		return domain.CatererInvitationPreview{}, errSchoolTransactionRequired
	}
	invitation, err := c.redeemable(ctx, token)
	if err != nil {
		return domain.CatererInvitationPreview{}, err
	}
	preview := domain.CatererInvitationPreview{Email: invitation.Email, CatererName: invitation.CatererName, ExpiresAt: invitation.ExpiresAt}
	school, found, err := c.schools.FindSchool(ctx, invitation.TenantID)
	if err != nil {
		return domain.CatererInvitationPreview{}, err
	}
	if found {
		preview.SchoolName = strings.TrimSpace(school.Name)
	}
	account, found, _, err := c.store.FindLoginAccountByEmail(ctx, invitation.Email)
	if err != nil || !found {
		return preview, err
	}
	_, isCaterer, _, err := c.store.FindCatererOfAccount(ctx, account.ID)
	preview.HasCatererAccount, preview.AddressTaken = isCaterer, !isCaterer
	return preview, err
}

// AcceptCatererInvitation spends the invitation in the school's transaction
// the caller opened. A new address gets an account and a caterer of the
// invited name; the account of an existing caterer confirms with its
// password and adds the school. An address of another kind of account is
// refused. The school's supply by the caterer starts with it, so the
// caterer never exists without its school.
func (c *CatererAccess) AcceptCatererInvitation(ctx context.Context, token string, registration identityaccess.CatererRegistration) (identityaccess.CatererAcceptance, error) {
	ctx = c.attach(ctx)
	if !c.inSchool(ctx) {
		return identityaccess.CatererAcceptance{}, catererError(failed("accept caterer invitation", errSchoolTransactionRequired))
	}
	accepted, err := c.accept(ctx, token, domain.CatererRegistration(registration))
	acceptance := identityaccess.CatererAcceptance(accepted)
	if err != nil {
		return identityaccess.CatererAcceptance{}, catererError(failed("accept caterer invitation", err))
	}
	c.logger.Info("caterer invitation accepted",
		slog.Int64("account_id", acceptance.AccountID),
		slog.Int64("caterer_id", acceptance.CatererID),
		slog.Int64("tenant_id", acceptance.TenantID))
	return acceptance, nil
}

// accept spends the invitation and links the caterer to the school.
func (c *CatererAccess) accept(ctx context.Context, token string, registration domain.CatererRegistration) (domain.CatererAcceptance, error) {
	invitation, err := c.redeemable(ctx, token)
	if err != nil {
		return domain.CatererAcceptance{}, err
	}
	// The school's own transaction holds it; a deleted school is refused.
	if school, found, schoolErr := c.schools.FindSchool(ctx, invitation.TenantID); schoolErr != nil {
		return domain.CatererAcceptance{}, schoolErr
	} else if !found || school.Deleted {
		return domain.CatererAcceptance{}, domain.ErrInvitationTenantDeleted
	}
	accountID, catererID, err := c.catererAccount(ctx, invitation, registration)
	if err != nil {
		return domain.CatererAcceptance{}, err
	}
	accepted, _, err := c.store.AcceptCatererInvitation(ctx, invitation.ID, accountID, c.now())
	switch {
	case err != nil:
		return domain.CatererAcceptance{}, err
	case !accepted:
		return domain.CatererAcceptance{}, domain.ErrInvitationUsed
	}
	// The supply starts with the acceptance; a school has one running
	// supply (uq_caterer_school_active).
	if _, err := c.store.InsertCatererSchool(ctx, catererID, invitation.DeliveryAddress, accountID, c.now()); err != nil {
		return domain.CatererAcceptance{}, err
	}
	return domain.CatererAcceptance{AccountID: accountID, CatererID: catererID, TenantID: invitation.TenantID}, nil
}

// catererAccount finds or creates the account and caterer the invitation
// goes to.
func (c *CatererAccess) catererAccount(ctx context.Context, invitation domain.CatererInvitation, registration domain.CatererRegistration) (int64, int64, error) {
	account, found, _, err := c.store.LockLoginAccountByEmail(ctx, invitation.Email)
	if err != nil {
		return 0, 0, err
	}
	if found {
		caterer, isCaterer, _, err := c.store.FindCatererOfAccount(ctx, account.ID)
		switch {
		case err != nil:
			return 0, 0, err
		case !isCaterer:
			return 0, 0, domain.ErrCatererAddressTaken
		}
		if valid, verifyErr := c.verifier.VerifyPassword(registration.Password, account.PasswordHash); verifyErr != nil || !valid {
			return 0, 0, identityaccess.ErrInvalidCredentials
		}
		return account.ID, caterer.ID, nil
	}
	if registration.Password != registration.ConfirmPassword {
		return 0, 0, domain.ErrPasswordMismatch
	}
	if err := c.passwords.ValidatePasswordStrength(registration.Password); err != nil {
		return 0, 0, err
	}
	hash, err := c.passwords.HashPassword(registration.Password)
	if err != nil {
		return 0, 0, err
	}
	created, _, err := c.store.InsertAccount(ctx, invitation.Email, hash)
	if err != nil {
		return 0, 0, err
	}
	caterer, _, err := c.store.InsertCaterer(ctx, invitation.CatererName)
	if err != nil {
		return 0, 0, err
	}
	if _, err := c.store.InsertCatererAccount(ctx, created.ID, caterer.ID); err != nil {
		return 0, 0, err
	}
	return created.ID, caterer.ID, nil
}

// redeemable resolves the school's invitation by its token and says why it
// cannot be accepted otherwise.
func (c *CatererAccess) redeemable(ctx context.Context, token string) (domain.CatererInvitation, error) {
	if strings.TrimSpace(token) == "" {
		return domain.CatererInvitation{}, domain.ErrInvitationNotFound
	}
	invitation, found, _, err := c.store.FindCatererInvitationByTokenHash(ctx, domain.HashInvitationToken(token))
	switch {
	case err != nil:
		return domain.CatererInvitation{}, err
	case !found:
		return domain.CatererInvitation{}, domain.ErrInvitationNotFound
	}
	return invitation, invitation.Redeemable(c.now())
}

// CatererSchools names the caterer of an account and the schools it
// supplies now. It is the caterer's one read across schools (ADR 0077, in
// the shape of ADR 0084): the global identity read, every query filtered on
// the account or its caterer. The caterer portal opens each school on its
// own.
func (c *CatererAccess) CatererSchools(ctx context.Context, accountID int64) (identityaccess.Caterer, []int64, error) {
	caterer, schools, err := c.catererSchools(c.attach(ctx), accountID)
	return identityaccess.Caterer(caterer), schools, catererError(err)
}

func (c *CatererAccess) catererSchools(ctx context.Context, accountID int64) (domain.Caterer, []int64, error) {
	var (
		caterer domain.Caterer
		schools []int64
	)
	err := globalIdentityRead(ctx, c.runtime, func(readCtx context.Context) error {
		found, _, err := c.findCaterer(readCtx, accountID, &caterer)
		switch {
		case err != nil:
			return err
		case !found:
			return domain.ErrAccountNotCaterer
		}
		schools, _, err = c.store.ListCatererSchools(readCtx, caterer.ID)
		if err != nil {
			return err
		}
		// School liveness belongs to Organisation & Tenancy. Resolve the
		// linked schools in bulk before the portal opens any tenant.
		active, err := c.schools.ListActiveSchoolsByID(readCtx, schools)
		if err != nil {
			return err
		}
		live := make(map[int64]bool, len(active))
		for _, school := range active {
			live[school.ID] = true
		}
		schools = slices.DeleteFunc(schools, func(id int64) bool { return !live[id] })
		return nil
	})
	if err != nil {
		return domain.Caterer{}, nil, failed("find caterer schools", err)
	}
	return caterer, schools, nil
}

func (c *CatererAccess) findCaterer(ctx context.Context, accountID int64, into *domain.Caterer) (bool, domain.OperationStats, error) {
	caterer, found, stats, err := c.store.FindCatererOfAccount(ctx, accountID)
	*into = caterer
	return found, stats, err
}

// LoginCatererWithAudit signs a caterer account in to the caterer portal.
// The session row is pinned to the first school the caterer supplies for its
// foreign key and RLS; the token itself carries no school. A caterer without
// a school, and any other account, is refused.
func (c *CatererAccess) LoginCatererWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := c.login(c.attach(ctx), email, password, ipAddress, userAgent)
	return access, refresh, catererError(err)
}

func (c *CatererAccess) login(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	account, err := c.auth.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}
	_, schools, err := c.catererSchools(ctx, account.ID)
	if err != nil {
		c.auth.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Not a caterer")
		return "", "", err
	}
	if len(schools) == 0 {
		// No school to file the event at: the refusal is logged only.
		c.logger.Info("caterer login refused: no school", slog.Int64("account_id", account.ID))
		return "", "", failed("caterer login", domain.ErrCatererWithoutSchool)
	}
	// A caterer has no school membership; the login is filed at the school
	// its session is pinned to.
	ctx = c.runtime.WithTenantID(ctx, schools[0])
	session, err := c.auth.createRefreshSession(ctx, account, schools[0], domain.ScopeCaterer)
	if err != nil {
		return "", "", err
	}
	access, refresh := buildClaims(account, session, buildCatererMetadata(account), email)
	return c.auth.generateAndLogTokens(ctx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
}

// buildCatererMetadata is the claims material of a caterer token: the
// caterer role name a token needs, no permission, no school. The portal
// resolves the caterer and its schools on every request.
func buildCatererMetadata(account domain.LoginAccount) *domain.AccountClaimsPayload {
	return &domain.AccountClaimsPayload{RoleNames: []string{domain.CatererRoleName}, Username: account.Username, Scope: domain.ScopeCaterer}
}

// newInvitationToken is a random link token of 32 bytes.
func newInvitationToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func publicCatererInvitation(invitation domain.CatererInvitation) identityaccess.CatererInvitation {
	return identityaccess.CatererInvitation{
		ID: invitation.ID, Email: invitation.Email, CatererName: invitation.CatererName, DeliveryAddress: invitation.DeliveryAddress,
		ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, RevokedAt: invitation.RevokedAt, CreatedAt: invitation.CreatedAt,
	}
}

var catererSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrAccountNotCaterer, identityaccess.ErrAccountNotCaterer},
	{domain.ErrCatererWithoutSchool, identityaccess.ErrCatererWithoutSchool},
	{domain.ErrCatererAddressTaken, identityaccess.ErrCatererAddressTaken},
	{domain.ErrInvitationRevoked, identityaccess.ErrInvitationRevoked},
	{domain.ErrInvalidCatererInvitation, identityaccess.ErrInvalidCatererInvitation},
	{domain.ErrCatererAlreadySupplies, identityaccess.ErrCatererAlreadySupplies},
}

// catererError translates the caterer sentinels and leaves credentials and
// invitations to the mappings of their flows.
func catererError(err error) error {
	for _, sentinel := range catererSentinels {
		if errors.Is(err, sentinel.internal) {
			return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
		}
	}
	if errors.Is(err, identityaccess.ErrInvalidCredentials) || errors.Is(err, domain.ErrAccountNotFound) || errors.Is(err, identityaccess.ErrAccountInactive) {
		return authenticationError(err)
	}
	return invitationError(err)
}

// refuseCatererAccount keeps a caterer account out of schools and families
// (ADR 0077): an invitation to its address answers like an address that is
// taken.
func refuseCatererAccount(ctx context.Context, logins ports.AccountLoginStore, accountID int64, op string) error {
	_, isCaterer, _, err := logins.FindCatererOfAccount(ctx, accountID)
	if err != nil {
		return failed(op, err)
	}
	if isCaterer {
		return failed(op, identityaccess.ErrEmailAlreadyExists)
	}
	return nil
}
