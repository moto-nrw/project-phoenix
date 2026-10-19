package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type MessageThreadAccess interface{ messaging.MessageThreadAccess }
type MessageThreadAccessCallbacks struct {
	FindByStudentGuardian func(ctx context.Context, studentID int64, guardianAccountID int64) (*ParentMessageThread, error)
	GetOrCreate           func(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64) (*ParentMessageThread, error)
}

func NewMessageThreadAccess(c MessageThreadAccessCallbacks) MessageThreadAccess {
	return messageThreadAccessBinding{c}
}

type messageThreadAccessBinding struct{ callbacks MessageThreadAccessCallbacks }

func (p messageThreadAccessBinding) FindByStudentGuardian(ctx context.Context, studentID int64, guardianAccountID int64) (*messaging.ParentMessageThread, error) {
	result0, result1 := p.callbacks.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	return messagingValuePointer(result0, messagingToAppParentMessageThread), result1
}
func (p messageThreadAccessBinding) GetOrCreate(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64) (*messaging.ParentMessageThread, error) {
	result0, result1 := p.callbacks.GetOrCreate(ctx, tenantID, studentID, guardianAccountID)
	return messagingValuePointer(result0, messagingToAppParentMessageThread), result1
}
