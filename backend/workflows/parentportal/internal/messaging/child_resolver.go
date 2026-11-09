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

func (c children) WithPermittedChild(ctx context.Context, accountID, studentID int64, permission string, between func(*care.Child) error, read func(context.Context, *care.Child) error) error {
	return (*c.flows).WithPermittedChild(ctx, accountID, studentID, permission, between, read)
}

func (c children) WithPermittedChildForWrite(ctx context.Context, accountID, studentID int64, permission string, between func(*care.Child) error, write func(context.Context, *care.Child) error) error {
	return (*c.flows).WithPermittedChildForWrite(ctx, accountID, studentID, permission, between, write)
}
