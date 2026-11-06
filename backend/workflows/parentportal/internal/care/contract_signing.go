package care

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Signing a care contract in the parents portal (#4679): the parents read
// the contract the Träger sent them and sign it with a drawn or typed
// signature. Reading needs what the inbox needs; signing binds the family,
// so the relationship must also grant submitting an enrollment for the
// child (parent_portal.enrollment.submit). The contract must have been sent
// to the account's address; the Betreuungsvertrag checks that and the child
// in the Träger transaction.

var (
	// ErrContractSignatureInvalid reports a signature image or typed name
	// the checks refuse.
	ErrContractSignatureInvalid = errors.New("parent: contract signature invalid")
	// ErrContractNotSignable reports a contract that waits for no signature.
	ErrContractNotSignable = errors.New("parent: contract cannot be signed")
	// ErrContractWithdrawalNoticeRequired reports a signature without the
	// confirmation of the withdrawal notice.
	ErrContractWithdrawalNoticeRequired = errors.New("parent: withdrawal notice must be confirmed")
)

// ContractSignatureInput is the signature as the parents give it.
type ContractSignatureInput struct {
	Method             string `json:"method"`
	ImagePNG           string `json:"image_png"`
	TypedName          string `json:"typed_name"`
	NoticeAcknowledged bool   `json:"withdrawal_notice_acknowledged"`
	EarlyStart         bool   `json:"early_start_requested"`
}

// ContractSigningView is the contract as the Betreuungsvertrag shows it to
// the parents before and after signing: the same document the one-time mail
// link shows, passed through unchanged so both ways read one source.
type ContractSigningView = json.RawMessage

// ContractSigningView reads a contract of the inbox to sign.
func (s *Service) ContractSigningView(ctx context.Context, accountID, organizationID, contractID int64) (ContractSigningView, error) {
	return s.contractSigning(ctx, accountID, organizationID, contractID, func(txCtx context.Context, reader BillingDocumentReader) (ContractSigningView, error) {
		return s.ContractDocuments.ContractSigningView(txCtx, organizationID, contractID, reader)
	})
}

// SignContract signs a contract of the inbox.
func (s *Service) SignContract(ctx context.Context, accountID, organizationID, contractID int64, input ContractSignatureInput) (ContractSigningView, error) {
	return s.contractSigning(ctx, accountID, organizationID, contractID, func(txCtx context.Context, reader BillingDocumentReader) (ContractSigningView, error) {
		return s.ContractDocuments.SignContract(txCtx, organizationID, contractID, reader, input)
	})
}

func (s *Service) contractSigning(ctx context.Context, accountID, organizationID, contractID int64,
	act func(context.Context, BillingDocumentReader) (ContractSigningView, error)) (ContractSigningView, error) {
	traeger, reader, err := s.contractSigningAccess(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(traeger, func(t Traeger) bool { return t.ID == organizationID }) || contractID <= 0 {
		return nil, ErrBillingDocumentNotFound
	}
	var view ContractSigningView
	err = s.TraegerRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var actErr error
		view, actErr = act(txCtx, reader)
		return actErr
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// contractSigningAccess is the inbox access of the account narrowed to the
// children whose relationship also grants submitting an enrollment.
func (s *Service) contractSigningAccess(ctx context.Context, accountID int64) ([]Traeger, BillingDocumentReader, error) {
	if s.ContractDocuments == nil || s.TraegerRuntime == nil || s.SchoolTraeger == nil || s.ParentAccounts == nil {
		return nil, BillingDocumentReader{}, errors.New("parent: contract signing is not configured")
	}
	var access documentAccess
	var email string
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		children, err := s.ListChildrenForAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		signing := make([]*ChildSummary, 0, len(children))
		for _, child := range children {
			if child != nil && childHasPermission(child, GuardianPermissionEnrollmentSubmit) {
				signing = append(signing, child)
			}
		}
		if access, err = s.documentAccess(adminCtx, signing); err != nil || len(access.traeger) == 0 {
			return err
		}
		email, err = s.ParentAccounts.AccountEmail(adminCtx, accountID)
		return err
	})
	if err != nil {
		return nil, BillingDocumentReader{}, fmt.Errorf("parent: resolve contract signing: %w", err)
	}
	if len(access.traeger) == 0 {
		return nil, BillingDocumentReader{}, ErrGuardianPermissionDenied
	}
	return access.traeger, BillingDocumentReader{AccountID: accountID, Email: strings.TrimSpace(email), Children: access.permittedChildren()}, nil
}
