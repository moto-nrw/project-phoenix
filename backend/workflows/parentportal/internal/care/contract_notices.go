package care

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Withdrawal and termination of a care contract in the parents portal
// (#4682, § 312k BGB). The parents open a contract from their inbox, read
// what it allows today, and end it in two steps: „Verträge hier kündigen“
// opens the page, „jetzt kündigen“ declares. Two facts must hold:
//
//   - The relationship: the account holds parent_portal.enrollment.submit
//     for the contract's child, besides the inbox right
//     parent_portal.enrollments.view. Who may bind the family with an
//     enrollment may also end the contract it led to; the inbox right alone
//     only reads.
//   - The payer: the contract was sent to the account's own address.
//
// The Betreuungsvertrag decides and records the declaration in the Träger
// transaction the flow opens; its confirmation goes to the account's address.

// The refusals of a declaration, as the Betreuungsvertrag reports them.
var (
	ErrContractNotEndable               = errors.New("parent: the contract is not active")
	ErrContractWithdrawalClosed         = errors.New("parent: the withdrawal period is over")
	ErrContractSpecialTerminationClosed = errors.New("parent: the contract has no special termination")
	ErrContractNoticeTemplateMissing    = errors.New("parent: the Träger has no confirmation template")
	ErrContractNoticeInvalid            = errors.New("parent: the declaration is invalid")
)

// ContractNoticeInput is the parents' declaration: kind "kuendigung" with
// termination_kind "ordentlich", "sonderkuendigung" or "ausserordentlich"
// (with the parents' reason, § 312k Abs. 2 BGB), or kind "widerruf".
type ContractNoticeInput struct {
	ExpectedEndsOn  calendar.Date `json:"expected_ends_on"`
	Kind            string        `json:"kind"`
	TerminationKind string        `json:"termination_kind"`
	Reason          string        `json:"reason"`
}

// ContractNotice is a declaration made, with its confirmation as sent.
type ContractNotice struct {
	DispatchedOn     *calendar.Date `json:"dispatched_on,omitempty"`
	Kind             string         `json:"kind"`
	TerminationKind  string         `json:"termination_kind,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	Channel          string         `json:"channel"`
	DeclaredAt       time.Time      `json:"declared_at"`
	ReceivedOn       calendar.Date  `json:"received_on"`
	EndsOn           *calendar.Date `json:"ends_on"`
	Recipient        string         `json:"recipient"`
	ConfirmationText string         `json:"confirmation_text"`
}

// ContractNoticeOptions is what the parents can declare today, or the
// declaration made.
type ContractNoticeOptions struct {
	ContractID      int64           `json:"contract_id"`
	OrganizationID  int64           `json:"organization_id"`
	TraegerName     string          `json:"traeger_name"`
	Status          string          `json:"status"`
	ChildName       string          `json:"child_name"`
	Modules         string          `json:"modules"`
	SchoolName      string          `json:"school_name"`
	Terms           string          `json:"terms"`
	OrdinaryEndsOn  *calendar.Date  `json:"ordinary_ends_on"`
	SpecialEndsOn   *calendar.Date  `json:"special_ends_on"`
	WithdrawalUntil *calendar.Date  `json:"withdrawal_until"`
	EarlyStart      bool            `json:"early_start"`
	Notice          *ContractNotice `json:"notice"`
}

// ContractNotices is the port to the Betreuungsvertrag's declarations in the
// Träger transaction the flow opens.
type ContractNotices interface {
	ContractNoticeOptions(ctx context.Context, organizationID, contractID int64, reader BillingDocumentReader) (ContractNoticeOptions, error)
	DeclareContractNotice(ctx context.Context, organizationID, contractID int64, reader BillingDocumentReader, traeger string,
		input ContractNoticeInput) (ContractNoticeOptions, error)
}

// ContractNoticeOptions reads what the account can declare on a contract
// sent to it.
func (s *Service) ContractNoticeOptions(ctx context.Context, accountID, organizationID, contractID int64) (ContractNoticeOptions, error) {
	return s.contractNotice(ctx, accountID, organizationID, contractID,
		func(txCtx context.Context, reader BillingDocumentReader, _ Traeger) (ContractNoticeOptions, error) {
			return s.ContractNotices.ContractNoticeOptions(txCtx, organizationID, contractID, reader)
		})
}

// DeclareContractNotice withdraws or terminates a contract sent to the
// account.
func (s *Service) DeclareContractNotice(ctx context.Context, accountID, organizationID, contractID int64, input ContractNoticeInput) (ContractNoticeOptions, error) {
	return s.contractNotice(ctx, accountID, organizationID, contractID,
		func(txCtx context.Context, reader BillingDocumentReader, traeger Traeger) (ContractNoticeOptions, error) {
			return s.ContractNotices.DeclareContractNotice(txCtx, organizationID, contractID, reader, traeger.Name, input)
		})
}

type contractNoticeStep func(txCtx context.Context, reader BillingDocumentReader, traeger Traeger) (ContractNoticeOptions, error)

// contractNotice resolves the children the account may act for at the
// Träger and runs step in that Träger's transaction. An account without such
// a child at any Träger gets ErrGuardianPermissionDenied; a Träger outside
// its relationships, ErrBillingDocumentNotFound.
func (s *Service) contractNotice(ctx context.Context, accountID, organizationID, contractID int64, step contractNoticeStep) (ContractNoticeOptions, error) {
	if s.ContractNotices == nil || s.TraegerRuntime == nil || s.SchoolTraeger == nil || s.ParentAccounts == nil {
		return ContractNoticeOptions{}, errors.New("parent: contract notices are not configured")
	}
	var access documentAccess
	var email string
	err := s.billingDocumentAccess(ctx, accountID, func(adminCtx context.Context, children []*ChildSummary) error {
		var err error
		if access, err = s.documentAccessFor(adminCtx, children, GuardianPermissionEnrollmentSubmit); err != nil || len(access.traeger) == 0 {
			return err
		}
		email, err = s.ParentAccounts.AccountEmail(adminCtx, accountID)
		return err
	})
	if err != nil {
		return ContractNoticeOptions{}, fmt.Errorf("parent: resolve contract notice: %w", err)
	}
	if len(access.traeger) == 0 {
		return ContractNoticeOptions{}, ErrGuardianPermissionDenied
	}
	index := slices.IndexFunc(access.traeger, func(t Traeger) bool { return t.ID == organizationID })
	if index < 0 || contractID <= 0 {
		return ContractNoticeOptions{}, ErrBillingDocumentNotFound
	}
	reader := BillingDocumentReader{AccountID: accountID, Email: email, Children: access.permittedChildren()}
	var options ContractNoticeOptions
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var stepErr error
		options, stepErr = step(txCtx, reader, access.traeger[index])
		return stepErr
	})
	if err != nil {
		return ContractNoticeOptions{}, err
	}
	options.OrganizationID, options.TraegerName = organizationID, access.traeger[index].Name
	return options, nil
}

// documentAccessFor keeps the children whose relationship grants permission
// and resolves their Träger.
func (s *Service) documentAccessFor(ctx context.Context, children []*ChildSummary, permission string) (documentAccess, error) {
	permitted := make([]*ChildSummary, 0, len(children))
	for _, child := range children {
		if child != nil && childHasPermission(child, permission) {
			permitted = append(permitted, child)
		}
	}
	return s.documentAccess(ctx, permitted)
}
