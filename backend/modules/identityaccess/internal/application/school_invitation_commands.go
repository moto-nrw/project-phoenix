package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CreateInvitation stores the invitation and its mail in the same transaction.
// The worker sends only committed entries: staff import may create invitations
// mid-transaction, and a rolled-back link must never reach an inbox.
func (s *SchoolInvitation) CreateInvitation(ctx context.Context, request domain.SchoolInvitationRequest) (result domain.SchoolInvitation, err error) {
	tenantID := request.TenantID
	if tenantID == 0 {
		tenantID = s.runtime.TenantID(ctx)
	}
	if tenantID <= 0 {
		return result, failed(opCreateInvitation, domain.ErrTenantRequired)
	}
	err = s.runtime.RunInTx(s.runtime.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
		result, err = s.createInvitation(txCtx, request)
		return err
	})
	return result, err
}

func (s *SchoolInvitation) ListPendingInvitations(ctx context.Context) (value0 []domain.SchoolInvitation, err error) {
	err = s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitListPendingInvitations(txCtx)
		return err
	})
	return
}

func (s *SchoolInvitation) RevokeInvitation(ctx context.Context, invitationID, actorAccountID int64) (err error) {
	err = s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = s.inUnitRevokeInvitation(txCtx, invitationID, actorAccountID)
		return err
	})
	return
}

func (s *SchoolInvitation) InvitationDeliverySent(ctx context.Context, id int64) (value0 bool, err error) {
	err = s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitInvitationDeliverySent(txCtx, id)
		return err
	})
	return
}

// FindFormerEmployment answers for the school in context, before any
// invitation exists, which former employment accepting one for the address
// would continue. An address without an account answers like one without a
// former employment here. It reads on the caller's school unit (the request
// transaction) and starts none of its own.
func (s *SchoolInvitation) FindFormerEmployment(ctx context.Context, email string) (domain.FormerEmployment, bool, error) {
	address, err := invitationAddress(email)
	if err != nil {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, err)
	}
	if s.runtime.TenantID(ctx) <= 0 {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, domain.ErrTenantRequired)
	}
	if !s.runtime.HasTransaction(ctx) || s.runtime.IsAdminTx(ctx) {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, errSchoolUnitRequired)
	}
	account, exists, _, err := s.logins.FindLoginAccountByEmail(ctx, address)
	if err != nil {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, err)
	}
	if !exists {
		return domain.FormerEmployment{}, false, nil
	}
	former, found, err := s.identity.FindFormerEmployment(ctx, account.ID)
	if err != nil {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, err)
	}
	return former, found, nil
}

// errSchoolUnitRequired refuses a former-employment question outside a school
// transaction: the person and membership reads rely on its RLS scope.
var errSchoolUnitRequired = errors.New("former employment lookup requires the school's transaction")

// invitationAddress is the normalized address an invitation names.
func invitationAddress(raw string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(raw))
	switch {
	case email == "":
		return "", fmt.Errorf("email is required")
	case len(email) > maxInvitationEmailLength:
		return "", fmt.Errorf("email address too long")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return "", fmt.Errorf("invalid email address")
	}
	return parsed.Address, nil
}
