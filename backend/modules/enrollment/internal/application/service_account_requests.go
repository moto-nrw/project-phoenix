package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// AccountRequests reads linked requests and pre-account submissions matching the
// account's identity email, supplied by the trusted parent workflow.
// Cross-school callers must supply their authorized admin transaction.
func (m *Service) AccountRequests(ctx context.Context, accountID int64, accountEmail string) ([]enrollment.AccountRequest, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	return m.store.AccountRequests(ctx, accountID, strings.ToLower(strings.TrimSpace(accountEmail)))
}
