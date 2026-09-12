package compose

import (
	"context"
	"errors"

	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

type reviewScope interface {
	Scope(context.Context) (reviewidentity.Scope, error)
}

type access struct {
	principal func(context.Context) reviewidentity.Principal
	policy    reviewScope
}

// NewAccess requires request identity; the optional review policy controls
// whether review_access is included, independently of queue authorization.
func NewAccess(principal func(context.Context) reviewidentity.Principal, policy reviewScope) (requestreview.Access, error) {
	if principal == nil {
		return nil, errors.New("request review access: request identity is required")
	}
	return access{principal: principal, policy: policy}, nil
}

func (a access) Caller(ctx context.Context) (requestreview.Caller, error) {
	return requestreview.Caller{ReviewsWriteQueues: a.principal(ctx).UsersUpdate}, nil
}

func (a access) ReviewAccess(ctx context.Context) (string, error) {
	if a.policy == nil {
		return "", nil
	}
	scope, err := a.policy.Scope(ctx)
	if err != nil {
		return "", err
	}
	return scope.AccessLevel(), nil
}
