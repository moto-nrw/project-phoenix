// Package settings exposes typed tenant-settings queries.
package settings

import (
	"context"
	"errors"

	configService "github.com/moto-nrw/project-phoenix/services/config"
)

var ErrQueriesUnavailable = errors.New("settings queries are unavailable")

type enrollmentReader interface {
	EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error)
}

// Queries is the public, typed read seam for tenant settings.
type Queries struct {
	enrollment enrollmentReader
}

func NewQueries(enrollment enrollmentReader) *Queries {
	return &Queries{enrollment: enrollment}
}

// EnrollmentEnabledForTenants reports the resolved enrollment master switch
// for every requested tenant. Missing overrides retain the registered platform
// default; lookup failures are returned to the caller.
func (q *Queries) EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error) {
	if q == nil || q.enrollment == nil {
		return nil, ErrQueriesUnavailable
	}
	return q.enrollment.EnrollmentEnabledForTenants(ctx, tenantIDs)
}

type Actor = configService.TenantActor
type ValueSetHook = configService.TenantValueSetHook
type LegalDocumentCleanup = configService.LegalDocumentCleanup

type Operations struct {
	tenant *configService.TenantOperations
}

func NewOperations(tenant *configService.TenantOperations) *Operations {
	return &Operations{tenant: tenant}
}

func (o *Operations) SetValueSetHook(hook ValueSetHook) {
	if o != nil && o.tenant != nil {
		o.tenant.SetValueSetHook(hook)
	}
}

func (o *Operations) Schema(ctx context.Context, permissions []string) (any, error) {
	if o == nil || o.tenant == nil {
		return nil, ErrQueriesUnavailable
	}
	return o.tenant.Schema(ctx, permissions)
}

func (o *Operations) Reveal(ctx context.Context, key string, permissions []string) (any, error) {
	if o == nil || o.tenant == nil {
		return nil, ErrQueriesUnavailable
	}
	return o.tenant.Reveal(ctx, key, permissions)
}

func (o *Operations) SetValue(ctx context.Context, actor Actor, key string, value any) error {
	if o == nil || o.tenant == nil {
		return ErrQueriesUnavailable
	}
	return o.tenant.SetValue(ctx, actor, key, value)
}

func (o *Operations) ResetValue(ctx context.Context, actor Actor, key string) error {
	if o == nil || o.tenant == nil {
		return ErrQueriesUnavailable
	}
	return o.tenant.ResetValue(ctx, actor, key)
}

func (o *Operations) PayrollStatus(ctx context.Context) (any, error) {
	if o == nil || o.tenant == nil {
		return nil, ErrQueriesUnavailable
	}
	return o.tenant.PayrollStatus(ctx)
}

func (o *Operations) LoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if o == nil || o.tenant == nil {
		return "", ErrQueriesUnavailable
	}
	return o.tenant.LoginImageURL(ctx, tenantID)
}

func (o *Operations) SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (string, error) {
	if o == nil || o.tenant == nil {
		return "", ErrQueriesUnavailable
	}
	return o.tenant.SetLoginImageURL(ctx, tenantID, imageURL)
}

func (o *Operations) ClearLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if o == nil || o.tenant == nil {
		return "", ErrQueriesUnavailable
	}
	return o.tenant.ClearLoginImageURL(ctx, tenantID)
}

func (o *Operations) SetLegalDocument(ctx context.Context, actor Actor, url string, cleanup LegalDocumentCleanup) error {
	if o == nil || o.tenant == nil {
		return ErrQueriesUnavailable
	}
	return o.tenant.SetLegalDocument(ctx, actor, url, cleanup)
}

func (o *Operations) DeleteLegalDocument(ctx context.Context, actor Actor, cleanup LegalDocumentCleanup) error {
	if o == nil || o.tenant == nil {
		return ErrQueriesUnavailable
	}
	return o.tenant.DeleteLegalDocument(ctx, actor, cleanup)
}

type ErrorKind uint8

const (
	ErrorInternal ErrorKind = iota
	ErrorNotFound
	ErrorInvalid
	ErrorForbidden
)

func ClassifyError(err error) ErrorKind {
	if errors.Is(err, configService.ErrTenantOperatorOnly) || errors.Is(err, configService.ErrDirectManagedSetting) || errors.Is(err, configService.ErrPermissionDenied) {
		return ErrorForbidden
	}
	if errors.Is(err, configService.ErrDefinitionNotFound) {
		return ErrorNotFound
	}
	if errors.Is(err, configService.ErrInvalidValue) || errors.Is(err, configService.ErrActiveLegalAGBPDF) {
		return ErrorInvalid
	}
	return ErrorInternal
}
