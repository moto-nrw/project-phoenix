package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

// AuditTransactionContext names the Audit handoff the root's tenant runtime
// attaches its tenant and transaction to, so audit appends join the unit of
// work they record.
func AuditTransactionContext() (func(context.Context, int64) context.Context, func(context.Context, any) context.Context) {
	return auditModels.WithTenantID, auditModels.WithTransaction
}

// tenantRuntimeBinder is the shape a composed module exposes when its own
// contract must not name the runtime package: it takes the unit of work as
// an opaque value and reports a value it cannot bind. The root owns the
// runtime, so the type check belongs here and fails at startup (#3364).
type tenantRuntimeBinder interface {
	SetTenantRuntime(any) error
}

// SetTenantRuntime wires the root's unit of work into every composed service
// that accepts one. Fields are discovered by reflection so a new service with
// a SetTenantRuntime method cannot be forgotten in a hand-maintained list. A
// service whose SetTenantRuntime takes the unit of work's own type receives
// it as such; one that takes an opaque value checks it itself (#3364).
func (f *Factory) SetTenantRuntime(runtime any) error {
	if runtime == nil {
		return errors.New("bind tenant runtime: no unit of work given")
	}
	if setter, ok := f.Settings.(interface {
		SetRuntime(settingsCompose.SettingsRuntime)
	}); ok {
		settingsRuntime, err := settingsCompose.BindTenantSettingsRuntime(f.settingsRuntimeDB, runtime)
		if err != nil {
			return fmt.Errorf("bind tenant runtime into Settings: %w", err)
		}
		setter.SetRuntime(settingsRuntime)
	}
	value := reflect.ValueOf(runtime)
	fields := reflect.ValueOf(f).Elem()
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		if !field.CanInterface() || isNilValue(field) {
			continue
		}
		if binder, ok := field.Interface().(tenantRuntimeBinder); ok {
			if err := binder.SetTenantRuntime(runtime); err != nil {
				return fmt.Errorf("bind tenant runtime into %s: %w", fields.Type().Field(i).Name, err)
			}
			continue
		}
		setTypedTenantRuntime(field, value)
	}
	return nil
}

// setTypedTenantRuntime calls field's SetTenantRuntime when it takes exactly
// the unit of work's type and returns nothing.
func setTypedTenantRuntime(field, runtime reflect.Value) {
	method := field.MethodByName("SetTenantRuntime")
	if !method.IsValid() {
		return
	}
	signature := method.Type()
	if signature.NumIn() != 1 || signature.NumOut() != 0 || signature.In(0) != runtime.Type() {
		return
	}
	method.Call([]reflect.Value{runtime})
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
