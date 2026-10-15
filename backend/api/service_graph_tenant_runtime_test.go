package api

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubUnitOfWork struct{ name string }

type runtimeRecorder struct{ bound string }

func (r *runtimeRecorder) SetTenantRuntime(runtime stubUnitOfWork) { r.bound = runtime.name }

type mistypedRuntimeTarget struct{}

func (mistypedRuntimeTarget) SetTenantRuntime(*stubUnitOfWork) error { return nil }

// interfaceField mirrors a serviceGraph field typed by an owner interface that
// does not name SetTenantRuntime: the setter lives on the dynamic value.
type interfaceField struct{ Service any }

func TestSetTypedTenantRuntimeBindsThroughAnInterfaceField(t *testing.T) {
	t.Parallel()

	recorder := &runtimeRecorder{}
	field := reflect.ValueOf(interfaceField{Service: recorder}).Field(0)

	require.NoError(t, setTypedTenantRuntime(field, reflect.ValueOf(stubUnitOfWork{name: "root"})))
	require.Equal(t, "root", recorder.bound)
}

func TestSetTypedTenantRuntimeRejectsAMistypedSetter(t *testing.T) {
	t.Parallel()

	field := reflect.ValueOf(interfaceField{Service: mistypedRuntimeTarget{}}).Field(0)

	require.Error(t, setTypedTenantRuntime(field, reflect.ValueOf(stubUnitOfWork{})))
}

func TestSetTypedTenantRuntimeSkipsServicesWithoutSetter(t *testing.T) {
	t.Parallel()

	field := reflect.ValueOf(interfaceField{Service: struct{}{}}).Field(0)

	require.NoError(t, setTypedTenantRuntime(field, reflect.ValueOf(stubUnitOfWork{})))
}
