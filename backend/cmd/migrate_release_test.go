package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestMigrationReleaseRequiresFullEmbeddedCommit(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("a1", 20)
	got, err := validateMigrationRelease(commit)
	require.NoError(t, err)
	require.Equal(t, commit, got)
	for _, invalid := range []string{"", "unknown", "ed76e88", strings.Repeat("g", 40), commit + "\n", strings.ToUpper(commit)} {
		_, err := validateMigrationRelease(invalid)
		require.ErrorContains(t, err, "embedded release commit")
	}
}

func TestStudentContractCommandLoadsOnlyMatchingReleaseEvidence(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("a", 40)
	for _, scenario := range []struct {
		name, contents, release, want string
	}{
		{"matching", `{"release_commit":"` + commit + `"}`, commit, ""},
		{"different release", `{"release_commit":"` + strings.Repeat("b", 40) + `"}`, commit, "does not match"},
		{"unversioned binary", `{}`, "", "embedded release commit"},
		{"invalid JSON", `{`, commit, "decode evidence"},
		{"policy injection", `{"minimum_rollback_window":0}`, commit, "unknown field"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "evidence.json")
			require.NoError(t, os.WriteFile(path, []byte(scenario.contents), 0600))
			command := &cobra.Command{}
			command.SetContext(t.Context())
			command.Flags().String(studentContractEvidenceFlag, path, "")
			ctx, err := studentContractCommandContext(command, scenario.release)
			if scenario.want != "" {
				require.ErrorContains(t, err, scenario.want)
				require.Nil(t, ctx)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, ctx)
		})
	}
}

func TestStudentContractEvidenceFlagAvailableForBothMigrationPhases(t *testing.T) {
	t.Parallel()
	for _, command := range []*cobra.Command{migrateCmd, migratePreflightCmd} {
		flag := command.Flags().Lookup(studentContractEvidenceFlag)
		require.NotNil(t, flag)
		require.Empty(t, flag.DefValue)
	}
	command := &cobra.Command{}
	command.SetContext(t.Context())
	command.Flags().String(studentContractEvidenceFlag, "", "")
	ctx, err := studentContractCommandContext(command, "")
	require.NoError(t, err)
	require.Equal(t, command.Context(), ctx, "ordinary migration commands do not require Contract evidence")
}
