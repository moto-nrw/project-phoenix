package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteSizedBuilderAnalyzerRejectsBroadCompositionLeaks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     string
		wantDetail string
	}{
		{
			name: "direct setup call",
			source: `package sample_test

import testutil "github.com/moto-nrw/project-phoenix/api/testutil"

func TestBroadSetup() { testutil.SetupAPITest(nil) }
`,
			wantDetail: "TestBroadSetup calls SetupAPITest outside a route/module builder",
		},
		{
			name: "direct aliased service factory construction",
			source: `package sample_test

import graph "github.com/moto-nrw/project-phoenix/services"

func TestBroadSetup() { graph.NewFactory(nil, nil, nil) }
`,
			wantDetail: "TestBroadSetup calls services.NewFactory outside a route/module builder",
		},
		{
			name: "route name does not excuse broad setup",
			source: `package sample_test
import (
 "net/http"
 testutil "github.com/moto-nrw/project-phoenix/api/testutil"
)
func setupSampleRoute() http.Handler { testutil.SetupAPITest(nil); return nil }
`,
			wantDetail: "setupSampleRoute calls SetupAPITest",
		},
		{
			name: "aliased service factory return",
			source: `package sample_test

import graph "github.com/moto-nrw/project-phoenix/services"

func setupSampleModule() *graph.Factory { return nil }
`,
			wantDetail: "setupSampleModule returns services.Factory",
		},
		{
			name: "wrapped service factory return",
			source: `package sample_test

import graph "github.com/moto-nrw/project-phoenix/services"

type moduleResult struct { Factory *graph.Factory }

func setupSampleModule() moduleResult { return moduleResult{} }
`,
			wantDetail: "setupSampleModule returns services.Factory",
		},
		{
			name: "any erased capability return",
			source: `package sample_test

type moduleResult struct { Capability any }

func setupSampleModule() moduleResult { return moduleResult{} }
`,
			wantDetail: "setupSampleModule returns an untyped capability",
		},
		{
			name: "misleading route builder",
			source: `package sample_test

import "github.com/uptrace/bun"

func setupSampleRoute() *bun.DB { return nil }
`,
			wantDetail: "setupSampleRoute does not return a router, handler, or API resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", tt.source)

			violations, err := routeSizedBuilderViolations(root)

			require.NoError(t, err)
			require.Condition(t, func() bool {
				return containsViolation(violations, tt.wantDetail)
			}, "violations %q do not contain %q", violations, tt.wantDetail)
		})
	}
}

func TestRouteSizedBuilderAnalyzerAcceptsNarrowBuilders(t *testing.T) {
	t.Parallel()

	source := `package sample_test

import (
	"net/http"

	sampleAPI "github.com/moto-nrw/project-phoenix/api/sample"
	"github.com/uptrace/bun"
)

type sampleModule struct {
	DB *bun.DB
	Run func() error
}

func setupSampleRoute() (*bun.DB, *sampleAPI.Resource) {
	return nil, nil
}

func buildSampleRouter() http.Handler { return nil }

func setupSampleModule() sampleModule { return sampleModule{} }
`
	root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", source)

	violations, err := routeSizedBuilderViolations(root)

	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestRouteSizedBuilderAnalyzerRejectsImportedFactoryCarrier(t *testing.T) {
	t.Parallel()

	root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test

import "github.com/moto-nrw/project-phoenix/helpers"

func setupSampleModule() helpers.Module { return helpers.Module{} }
`)
	writeRouteBuilderSource(t, root, "helpers/module.go", `package helpers

import graph "github.com/moto-nrw/project-phoenix/services"

type Module struct { Factory *graph.Factory }
`)

	violations, err := routeSizedBuilderViolations(root)

	require.NoError(t, err)
	require.Condition(t, func() bool {
		return containsViolation(violations, "setupSampleModule returns services.Factory")
	}, "violations %q do not reject the imported carrier", violations)
}

func TestRouteSizedBuilderAnalyzerRejectsImportedUntypedCarrier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		helper string
	}{
		{name: "direct any", helper: "type Module struct { Capability any }"},
		{name: "empty interface", helper: "type Module interface{}"},
		{name: "embedded any interface", helper: "type Module interface { any }"},
		{name: "nested any", helper: "type Hidden struct { Capability any }; type Module struct { Hidden Hidden }"},
		{name: "generic any", helper: "type Carrier[T any] struct { Capability any }; type Module Carrier[int]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test

import "github.com/moto-nrw/project-phoenix/helpers"

func setupSampleModule() helpers.Module { return helpers.Module{} }
`)
			writeRouteBuilderSource(t, root, "helpers/module.go", "package helpers\n\n"+tt.helper+"\n")

			violations, err := routeSizedBuilderViolations(root)

			require.NoError(t, err)
			require.Condition(t, func() bool {
				return containsViolation(violations, "setupSampleModule returns an untyped capability")
			}, "violations %q do not reject the imported untyped carrier", violations)
		})
	}
}

func TestRouteSizedBuilderAnalyzerRejectsTransitiveImportedUntypedCarrier(t *testing.T) {
	t.Parallel()

	root := writeRouteBuilderFixture(t, "api/sample/sample_test.go", `package sample_test

import "github.com/moto-nrw/project-phoenix/helpers"

func setupSampleModule() helpers.Module { return helpers.Module{} }
`)
	writeRouteBuilderSource(t, root, "helpers/module.go", `package helpers

import "github.com/moto-nrw/project-phoenix/hidden"

type Module struct { Hidden hidden.Hidden }
`)
	writeRouteBuilderSource(t, root, "hidden/hidden.go", `package hidden

type Hidden struct { Capability any }
`)

	violations, err := routeSizedBuilderViolations(root)

	require.NoError(t, err)
	require.Condition(t, func() bool {
		return containsViolation(violations, "setupSampleModule returns an untyped capability")
	}, "violations %q do not reject the transitive imported untyped carrier", violations)
}

func writeRouteBuilderFixture(t *testing.T, relativePath, source string) string {
	t.Helper()
	root := t.TempDir()
	writeRouteBuilderSource(t, root, relativePath, source)
	return root
}

func writeRouteBuilderSource(t *testing.T, root, relativePath, source string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
}

func containsViolation(violations []string, detail string) bool {
	for _, violation := range violations {
		if strings.Contains(violation, detail) {
			return true
		}
	}
	return false
}
