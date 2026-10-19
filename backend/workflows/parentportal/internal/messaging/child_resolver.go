package messaging

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type children struct{ flows **care.Service }

func NewChildResolver(flows **care.Service) ChildResolver { return children{flows: flows} }
func (c children) ResolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*care.Child, error) {
	return (*c.flows).ResolvePermittedChild(ctx, accountID, studentID, permission)
}
