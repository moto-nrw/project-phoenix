package contract

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeScriptModule_MatchesCheckedInFrontendCopy(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller should resolve the current test file")

	packageDir := filepath.Dir(currentFile)
	checkedInPath := filepath.Join(packageDir, "..", "..", "..", "frontend", "e2e", "contract.generated.ts")

	data, err := os.ReadFile(filepath.Clean(checkedInPath))
	require.NoError(t, err)

	assert.Equal(
		t,
		TypeScriptModule(),
		string(data),
		"frontend/e2e/contract.generated.ts is stale; run `cd backend && go run main.go e2e print-contract-ts > ../frontend/e2e/contract.generated.ts` and commit the generated file",
	)
}
