package behavior_test

import (
	"context"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The behaviour suites in this directory were written against the retained
// auth service's call shapes: int account ids, the audit-less login and
// refresh convenience calls, and the retained parent account model. Those
// shapes are gone with the ports (#3364); the flows behind them are the
// module's. The shims below keep the suites driving real behaviour without
// rewriting twenty thousand lines of assertions.

// retainedSessionCalls are the convenience shapes the suites call.
type retainedSessionCalls interface {
	Login(ctx context.Context, email, password string) (string, string, error)
	LoginParent(ctx context.Context, email, password string) (string, string, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, string, error)
	RevokeAllTokens(ctx context.Context, accountID int) error
	GetActiveTokens(ctx context.Context, accountID int) ([]*authModels.Token, error)
	CreateParentAccount(ctx context.Context, email, username, password string) (*authModels.AccountParent, error)
	GetParentAccountByID(ctx context.Context, id int) (*authModels.AccountParent, error)
	GetParentAccountByEmail(ctx context.Context, email string) (*authModels.AccountParent, error)
	UpdateParentAccount(ctx context.Context, account *authModels.AccountParent) error
	ActivateParentAccount(ctx context.Context, accountID int) error
	DeactivateParentAccount(ctx context.Context, accountID int) error
	ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*authModels.AccountParent, error)
}

// sessions is the module the shims delegate to.
func (s *fixtureOwnedAuthService) sessions() *identityaccess.Module { return s.module }

func (s *fixtureOwnedAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.sessions().LoginWithAudit(ctx, email, password, "", "", "")
}

func (s *fixtureOwnedAuthService) LoginParent(ctx context.Context, email, password string) (string, string, error) {
	return s.sessions().LoginParentWithAudit(ctx, email, password, "", "")
}

func (s *fixtureOwnedAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	return s.sessions().RefreshTokenWithAudit(ctx, refreshToken, "", "")
}

func (s *fixtureOwnedAuthService) RevokeAllTokens(ctx context.Context, accountID int) error {
	return s.sessions().RevokeAllTokensWithReason(ctx, int64(accountID), "administrative_revoke")
}

func (s *fixtureOwnedAuthService) GetActiveTokens(ctx context.Context, accountID int) ([]*authModels.Token, error) {
	active, err := s.sessions().ListActiveSessions(ctx, int64(accountID))
	if err != nil {
		return nil, err
	}
	tokens := make([]*authModels.Token, 0, len(active))
	for _, session := range active {
		token := &authModels.Token{
			Model:     modelBase.Model{ID: session.ID, CreatedAt: session.CreatedAt},
			AccountID: int64(accountID),
			Token:     session.Token,
			Expiry:    session.Expiry,
			Mobile:    session.Mobile,
		}
		token.Identifier = session.Identifier
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func parentAccountModel(record identityaccess.ParentAccount) *authModels.AccountParent {
	row := &authModels.AccountParent{Email: record.Email, Active: record.Active}
	row.ID = record.ID
	row.CreatedAt = record.CreatedAt
	row.UpdatedAt = record.UpdatedAt
	row.SetTenantID(record.TenantID)
	if record.Username != "" {
		username := record.Username
		row.Username = &username
	}
	return row
}

func (s *fixtureOwnedAuthService) CreateParentAccount(ctx context.Context, email, username, password string) (*authModels.AccountParent, error) {
	record, err := s.sessions().CreateParentAccount(ctx, email, username, password)
	if err != nil {
		return nil, err
	}
	return parentAccountModel(record), nil
}

func (s *fixtureOwnedAuthService) GetParentAccountByID(ctx context.Context, id int) (*authModels.AccountParent, error) {
	record, err := s.sessions().GetParentAccountByID(ctx, int64(id))
	if err != nil {
		return nil, err
	}
	return parentAccountModel(record), nil
}

func (s *fixtureOwnedAuthService) GetParentAccountByEmail(ctx context.Context, email string) (*authModels.AccountParent, error) {
	record, err := s.sessions().GetParentAccountByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return parentAccountModel(record), nil
}

func (s *fixtureOwnedAuthService) UpdateParentAccount(ctx context.Context, account *authModels.AccountParent) error {
	if account == nil {
		return identityaccess.ErrParentAccountNotFound
	}
	record := identityaccess.ParentAccount{
		ID: account.ID, TenantID: account.TenantID, Email: account.Email, Active: account.Active,
	}
	if account.Username != nil {
		record.Username = *account.Username
	}
	return s.sessions().UpdateParentAccount(ctx, record)
}

func (s *fixtureOwnedAuthService) ActivateParentAccount(ctx context.Context, accountID int) error {
	return s.sessions().ActivateParentAccount(ctx, int64(accountID))
}

func (s *fixtureOwnedAuthService) DeactivateParentAccount(ctx context.Context, accountID int) error {
	return s.sessions().DeactivateParentAccount(ctx, int64(accountID))
}

func (s *fixtureOwnedAuthService) ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*authModels.AccountParent, error) {
	var filter identityaccess.ParentAccountFilter
	if value, ok := filters["email"].(string); ok {
		filter.Email = value
	}
	if value, ok := filters["active"].(bool); ok {
		filter.Active = &value
	}
	records, err := s.sessions().ListParentAccounts(ctx, filter)
	if err != nil {
		return nil, err
	}
	accounts := make([]*authModels.AccountParent, 0, len(records))
	for _, record := range records {
		accounts = append(accounts, parentAccountModel(record))
	}
	return accounts, nil
}
