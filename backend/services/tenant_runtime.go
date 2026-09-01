package services

import (
	"context"
	"reflect"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func BindTenantRuntime(
	withinTenant func(context.Context, int64, func(context.Context, any) error) error,
	withinAdmin func(context.Context, func(context.Context, any) error) error,
	savepoints tenant.SavepointController,
	retryable func(error) bool,
) (tenant.UnitOfWork, error) {
	lockController, _ := savepoints.(interface {
		AcquireLock(context.Context, string, bool) error
	})
	var acquireLock func(context.Context, string, bool) error
	if lockController != nil {
		acquireLock = lockController.AcquireLock
	}
	runtime, err := tenant.NewUnitOfWork(
		withinTenant,
		withinAdmin,
		tenant.SavepointFunc(savepoints),
		retryable,
		acquireLock,
	)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	if detacher, ok := savepoints.(interface {
		ContextWithoutTransaction(context.Context) context.Context
	}); ok {
		runtime = runtime.WithTransactionDetacher(detacher.ContextWithoutTransaction)
	}
	if adapter, ok := savepoints.(interface {
		ContextWithTenant(context.Context, int64) context.Context
		ContextWithTransaction(context.Context, any) context.Context
	}); ok {
		runtime = runtime.WithContextAdapters(adapter.ContextWithTenant, adapter.ContextWithTransaction)
	}
	runtime = runtime.WithTransactionDetacher(func(ctx context.Context) context.Context {
		return auditModels.WithTransaction(ctx, nil)
	})
	runtime = runtime.WithContextAdapters(auditModels.WithTenantID, auditModels.WithTransaction)
	return runtime, nil
}

// ObserveUnitOfWorkPoolWait bridges the PostgreSQL adapter to the transaction
// observer without exposing the tenant package to composition callers.
func ObserveUnitOfWorkPoolWait(ctx context.Context, duration time.Duration) {
	tenant.ObservePoolWait(ctx, duration)
}

func ObserveUnitOfWorkLockWait(ctx context.Context, duration time.Duration) {
	tenant.ObserveLockWait(ctx, duration)
}

type tenantRuntimeSetter interface {
	SetTenantRuntime(tenant.UnitOfWork)
}

// SetTenantRuntime wires the runtime into every composed service that accepts
// one. Fields are discovered by reflection so a new service with a
// SetTenantRuntime method cannot be forgotten in a hand-maintained list.
func (f *Factory) SetTenantRuntime(runtime tenant.UnitOfWork) error {
	if setter, ok := f.Settings.(interface{ SetRuntime(config.Runtime) }); ok {
		setter.SetRuntime(newSettingsRuntime(f.settingsRuntimeDB, &runtime))
	}
	fields := reflect.ValueOf(f).Elem()
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		if !field.CanInterface() || isNilValue(field) {
			continue
		}
		if setter, ok := field.Interface().(tenantRuntimeSetter); ok {
			setter.SetTenantRuntime(runtime)
		}
	}
	return nil
}

// isNilValue treats an interface holding a nil pointer as nil, matching the
// "not composed" meaning of a nil field.
func isNilValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Interface:
		return v.IsNil() || isNilValue(v.Elem())
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	default:
		return false
	}
}
