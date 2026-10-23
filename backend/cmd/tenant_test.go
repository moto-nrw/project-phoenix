package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestTenantExportCommandFailureExitsNonzero(t *testing.T) {
	t.Parallel()
	if os.Getenv("PHX_EXPORT_FAILURE_CHILD") == "1" {
		RootCmd = tenantCommand(func(_ *cobra.Command, options organizationtenancy.TenantExportOptions) error {
			if err := os.Mkdir(options.Output, 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(options.Output, "partial.jsonl"), []byte("{}\n"), 0o600); err != nil {
				return err
			}
			return errors.New("synthetic export failure")
		})
		RootCmd.SetArgs([]string{"export", "--school", strconv.FormatInt(testpkg.UniqueTestTenantID(t), 10), "--out", os.Getenv("PHX_EXPORT_FAILURE_OUTPUT"), "--uploads", filepath.Dir(os.Getenv("PHX_EXPORT_FAILURE_OUTPUT"))})
		Execute() // the real root must turn RunE's failure into os.Exit(1)
		t.Fatal("command failure returned success")
	}
	out := filepath.Join(t.TempDir(), "failed")
	output, exit, err := testpkg.RunTestBinary("TestTenantExportCommandFailureExitsNonzero", []string{"PHX_EXPORT_FAILURE_CHILD=1", "PHX_EXPORT_FAILURE_OUTPUT=" + out})
	require.Error(t, err)
	require.Equal(t, 1, exit)
	require.Contains(t, string(output), "synthetic export failure")
	require.NotContains(t, string(output), "Tenant export complete")
	require.FileExists(t, filepath.Join(out, "partial.jsonl"))
	require.NoFileExists(t, filepath.Join(out, "COMPLETE"))
}
