package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// Pin the production root, not a test factory that happens to pass listen=true.
// Worker-only roots must not consume a dedicated LISTEN connection.
func TestRuntimeRealtimeListenerWiring(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	require.NoError(t, err)
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "NewPostgresRealtimeHub" {
			return true
		}
		require.Len(t, call.Args, 3)
		condition, ok := call.Args[2].(*ast.BinaryExpr)
		require.True(t, ok, "listener must depend on process mode")
		require.Equal(t, token.NEQ, condition.Op)
		process, ok := condition.X.(*ast.SelectorExpr)
		require.True(t, ok)
		require.Equal(t, "Process", process.Sel.Name)
		config, ok := process.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "config", config.Name)
		worker, ok := condition.Y.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "ServeWorkerOnly", worker.Name)
		found = true
		return true
	})
	require.True(t, found, "production root must construct the realtime transport")
}
