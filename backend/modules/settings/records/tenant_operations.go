package records

import "context"

// TenantOperationsService is the tenant settings HTTP application port.
type TenantOperationsService interface {
	Schema(context.Context, []string) (any, error)
	LoginImages
	Reveal(ctx context.Context, key string, permissions []string) (any, error)
	SetValue(ctx context.Context, tenantID, accountID int64, permissions []string, key string, value any) error
	ResetValue(ctx context.Context, tenantID, accountID int64, permissions []string, key string) error
	PayrollStatus(ctx context.Context) (any, error)
	SetLegalDocument(ctx context.Context, tenantID, accountID int64, permissions []string, documentURL string, cleanup func(context.Context, int64, string, string) (func(), error)) error
	DeleteLegalDocument(ctx context.Context, tenantID, accountID int64, permissions []string, cleanup func(context.Context, int64, string, string) (func(), error)) error
	ClassifyError(err error) string
}
