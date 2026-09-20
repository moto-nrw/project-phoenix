package cmd

import (
	"errors"
	"regexp"
)

// migrationReleaseCommit is embedded by the image build, never read from the
// evidence document or runtime environment. Local unversioned binaries cannot
// authorize a destructive Contract on an existing installation.
var migrationReleaseCommit string

var migrationReleasePattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func validateMigrationRelease(commit string) (string, error) {
	if !migrationReleasePattern.MatchString(commit) {
		return "", errors.New("student contract: full embedded release commit is required; use a versioned release build")
	}
	return commit, nil
}
