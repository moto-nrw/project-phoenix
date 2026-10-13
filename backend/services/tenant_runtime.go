package services

import (
	"fmt"
	"reflect"

	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantRuntime is the unit of work composed services run tenant work under.
// Package-local callers name it through this alias so composition tests stay
// free of the runtime package.
type TenantRuntime = tenant.UnitOfWork

type tenantRuntimeSetter interface {
	SetTenantRuntime(tenant.UnitOfWork)
}

// tenantRuntimeBinder is the shape a composed module exposes when its own
// contract must not name the runtime package: it takes the unit of work as
// an opaque value and reports a value it cannot bind. The root owns the
// runtime, so the type check belongs here and fails at startup (#3364).
type tenantRuntimeBinder interface {
	SetTenantRuntime(any) error
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
		switch setter := field.Interface().(type) {
		case tenantRuntimeSetter:
			setter.SetTenantRuntime(runtime)
		case tenantRuntimeBinder:
			if err := setter.SetTenantRuntime(runtime); err != nil {
				return fmt.Errorf("bind tenant runtime into %s: %w", fields.Type().Field(i).Name, err)
			}
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
