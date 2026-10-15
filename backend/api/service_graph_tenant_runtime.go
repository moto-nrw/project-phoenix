package api

import (
	"errors"
	"fmt"
	"reflect"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

// tenantRuntimeBinder is the shape a composed module exposes when its own
// contract must not name the runtime package: it takes the unit of work as
// an opaque value and reports a value it cannot bind. The root owns the
// runtime, so the type check belongs here and fails at startup (#3364).
type tenantRuntimeBinder interface {
	SetTenantRuntime(any) error
}

// bindTenantRuntime wires the root's unit of work into every composed service
// that accepts one. Fields are discovered by reflection so a new service with
// a SetTenantRuntime method cannot be forgotten in a hand-maintained list. A
// service whose SetTenantRuntime takes the unit of work's own type receives
// it as such; one that takes an opaque value checks it itself (#3364).
func (f *serviceGraph) bindTenantRuntime(runtime any) error {
	if runtime == nil {
		return errors.New("bind tenant runtime: no unit of work given")
	}
	if setter, ok := f.Settings.(interface {
		SetRuntime(settingsCompose.SettingsRuntime)
	}); ok {
		settingsRuntime, err := settingsCompose.BindTenantSettingsRuntime(f.database.Pool(), runtime)
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
		if err := setTypedTenantRuntime(field, value); err != nil {
			return fmt.Errorf("bind tenant runtime into %s: %w", fields.Type().Field(i).Name, err)
		}
	}
	return nil
}

// setTypedTenantRuntime calls field's SetTenantRuntime with the unit of
// work. A field without the method takes no runtime; a method of any other
// shape is a wiring error that fails startup instead of leaving the service
// unbound.
func setTypedTenantRuntime(field, runtime reflect.Value) error {
	method := reflect.ValueOf(field.Interface()).MethodByName("SetTenantRuntime")
	if !method.IsValid() {
		return nil
	}
	signature := method.Type()
	if signature.NumIn() != 1 || signature.NumOut() != 0 || !runtime.Type().AssignableTo(signature.In(0)) {
		return fmt.Errorf("SetTenantRuntime has signature %s, want func(%s)", signature, runtime.Type())
	}
	method.Call([]reflect.Value{runtime})
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
