package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteSizedBuilderAnalyzerFollowsCompositionGraph(t *testing.T) {
	t.Parallel()
	for _, constructor := range []string{"services.NewFactory", "services.NewFactoryForTestsWithFeedback", "stores.NewFactory", "stores.NewFactoryWithClock"} {
		t.Run(constructor, func(t *testing.T) {
			root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test
import "github.com/moto-nrw/project-phoenix/support"
func TestRoute() { support.BuildRoute() }
`)
			writeRouteBuilderSource(t, root, "support/helper.go", `package support
import (
 "github.com/moto-nrw/project-phoenix/services"
 stores "github.com/moto-nrw/project-phoenix/database/repositories"
)
var construct = `+constructor+`
func BuildRoute() { construct(nil) }
`)
			violations, err := routeSizedBuilderViolations(root)
			require.NoError(t, err)
			require.Condition(t, func() bool {
				return containsViolation(violations, "support/helper.go#construct references broad composition")
			}, "%v", violations)
		})
	}
}

func TestRouteSizedBuilderAnalyzerLimitsFullRouterException(t *testing.T) {
	t.Parallel()
	root := writeRouteBuilderFixture(t, "api/route_table_golden_test.go", `package api
func TestFullProductionRouterGolden() { WithRuntime(nil) }
`)
	writeRouteBuilderSource(t, root, "api/runtime.go", `package api
func WithRuntime(any) { newRuntime() }
func newRuntime() {}
`)
	violations, err := routeSizedBuilderViolations(root)
	require.NoError(t, err)
	require.Empty(t, violations)
	writeRouteBuilderSource(t, root, "api/normal_test.go", `package api
func TestNormal() { TestFullProductionRouterGolden() }
`)
	violations, err = routeSizedBuilderViolations(root)
	require.NoError(t, err)
	require.Condition(t, func() bool { return containsViolation(violations, "TestNormal references broad composition") }, "%v", violations)
}

func TestRouteSizedBuilderAnalyzerRejectsBroadInitializer(t *testing.T) {
	t.Parallel()
	root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test
import "github.com/moto-nrw/project-phoenix/services"
var hidden = services.NewFactory(nil, nil, nil)
`)
	violations, err := routeSizedBuilderViolations(root)
	require.NoError(t, err)
	require.NotEmpty(t, violations)
}

func TestRouteSizedBuilderAnalyzerRejectsImplicitAndMethodComposition(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, caller, support string }{
		{"receiver method", "support.NewBuilder().BuildRoute()", `type Builder struct{}
func NewBuilder() *Builder { return &Builder{} }
func (*Builder) BuildRoute() { services.NewFactory(nil, nil, nil) }`},
		{"package init", "support.BuildRoute()", `func init() { services.NewFactory(nil, nil, nil) }
func BuildRoute() {}`},
		{"unused package initializer", "support.BuildRoute()", `var hidden = services.NewFactory(nil, nil, nil)
func BuildRoute() {}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test
import "github.com/moto-nrw/project-phoenix/support"
func TestRoute() { `+test.caller+` }
`)
			writeRouteBuilderSource(t, root, "support/helper.go", `package support
import "github.com/moto-nrw/project-phoenix/services"
`+test.support)
			violations, err := routeSizedBuilderViolations(root)
			require.NoError(t, err)
			require.NotEmpty(t, violations)
		})
	}
}

func TestRouteSizedBuilderAnalyzerFollowsReturnedReceiverPackage(t *testing.T) {
	t.Parallel()
	root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test
import "github.com/moto-nrw/project-phoenix/support"
func TestRoute() { support.NewBuilder().BuildRoute() }
`)
	writeRouteBuilderSource(t, root, "support/helper.go", `package support
import "github.com/moto-nrw/project-phoenix/builders"
func NewBuilder() *builders.Builder { return &builders.Builder{} }
`)
	writeRouteBuilderSource(t, root, "builders/builder.go", `package builders
import "github.com/moto-nrw/project-phoenix/services"
type Builder struct{}
func (*Builder) BuildRoute() { services.NewFactory(nil, nil, nil) }
`)
	violations, err := routeSizedBuilderViolations(root)
	require.NoError(t, err)
	require.NotEmpty(t, violations)
}
