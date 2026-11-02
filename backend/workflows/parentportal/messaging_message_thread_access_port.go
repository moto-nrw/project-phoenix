package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type MessageThreadAccess interface{ messaging.MessageThreadAccess }
type MessageThreadAccessCallbacks struct {
	FindByStudentGuardian func(ctx context.Context, studentID int64, guardianAccountID int64, audience string) (*ParentMessageThread, error)
	GetOrCreate           func(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64, audience string) (*ParentMessageThread, error)
}

func NewMessageThreadAccess(c MessageThreadAccessCallbacks) MessageThreadAccess {
	return messageThreadAccessBinding{c}
}

type messageThreadAccessBinding struct{ callbacks MessageThreadAccessCallbacks }

func (p messageThreadAccessBinding) FindByStudentGuardian(ctx context.Context, studentID int64, guardianAccountID int64, audience string) (*messaging.ParentMessageThread, error) {
	return p.callbacks.FindByStudentGuardian(ctx, studentID, guardianAccountID, audience)
}
func (p messageThreadAccessBinding) GetOrCreate(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64, audience string) (*messaging.ParentMessageThread, error) {
	return p.callbacks.GetOrCreate(ctx, tenantID, studentID, guardianAccountID, audience)
}
