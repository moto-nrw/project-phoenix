package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
)

const studentContractEvidenceFlag = "student-contract-evidence"

func studentContractCommandContext(cmd *cobra.Command, embeddedCommit string) (context.Context, error) {
	path, err := cmd.Flags().GetString(studentContractEvidenceFlag)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return cmd.Context(), nil
	}
	release, err := validateMigrationRelease(embeddedCommit)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open student contract evidence: %w", err)
	}
	defer func() { _ = file.Close() }()
	evidence, err := migrations.ReadStudentContractEvidence(file)
	if err != nil {
		return nil, err
	}
	if evidence.ReleaseCommit != release {
		return nil, fmt.Errorf("student contract: evidence does not match the embedded release commit")
	}
	return migrations.WithStudentContractEvidence(cmd.Context(), evidence, release), nil
}
