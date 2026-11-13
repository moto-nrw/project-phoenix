package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

func TestTenantExportEmptyPersonCannotBecomeSchoolExport(t *testing.T) {
	t.Parallel()
	command := tenantCommand(func(_ *cobra.Command, _ organizationtenancy.TenantExportOptions) error {
		t.Fatal("explicitly empty person selection must not run a school-wide export")
		return nil
	})
	command.SetArgs([]string{"export", "--person", ""})
	require.ErrorContains(t, command.Execute(), "--person must not be empty")
}

func TestTenantExportOrganizationExcludesSchoolAndPerson(t *testing.T) {
	t.Parallel()
	var got organizationtenancy.TenantExportOptions
	command := tenantCommand(func(_ *cobra.Command, options organizationtenancy.TenantExportOptions) error {
		got = options
		return nil
	})
	organization := testpkg.UniqueTestTenantID(t)
	command.SetArgs([]string{"export", "--organization", strconv.FormatInt(organization, 10), "--out", "out", "--uploads", "uploads"})
	require.NoError(t, command.Execute())
	require.Equal(t, organization, got.OrganizationID)
	require.Zero(t, got.SchoolID)

	for _, args := range [][]string{
		{"export", "--organization", "7", "--school", "8", "--out", "out", "--uploads", "uploads"},
		{"export", "--organization", "7", "--person", "kind:1", "--out", "out", "--uploads", "uploads"},
	} {
		command := tenantCommand(func(_ *cobra.Command, _ organizationtenancy.TenantExportOptions) error {
			t.Fatal("a Träger package is never a school or person export")
			return nil
		})
		command.SetArgs(args)
		require.Error(t, command.Execute(), args)
	}
}
