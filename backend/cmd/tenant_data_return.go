package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/spf13/cobra"
)

// The operator side of the retrieval of a return package (Art. 30 Abs. 2
// Data Act, ADR 0203): release a checked package for HTTPS retrieval, issue
// further tokens, revoke them and purge the store. The commands run as the
// group role phoenix_data_return; the database functions check the operator
// and write the operator audit. Tokens and archive keys never go to
// stdout, which the container runtime logs: they go to a new private file.

// openDataReturn opens the operator side and answers how to close it.
type openDataReturn func(context.Context) (organizationtenancy.DataReturnRelease, func(), error)

func newTenantReturnCommand() *cobra.Command {
	return newTenantReturnCommandWith(openDataReturnRelease)
}

func newTenantReturnCommandWith(open openDataReturn) *cobra.Command {
	parent := &cobra.Command{Use: "return", Short: "Release a checked return package for HTTPS retrieval (Data Act)"}
	parent.AddCommand(newTenantReturnRelease(open), newTenantReturnGrant(open), newTenantReturnRevoke(open),
		newTenantReturnPurge(open), newTenantReturnList(open))
	return parent
}

func openDataReturnRelease(context.Context) (organizationtenancy.DataReturnRelease, func(), error) {
	db, err := transactionCompose.OpenDatabase()
	if err != nil {
		return nil, nil, errors.New("data return: database connection unavailable")
	}
	release, err := organizationCompose.NewDataReturnRelease(db.Pool(), owners.NewDataReturnCrypto(), organizationtenancy.DataReturnStoreDir)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return release, func() { _ = db.Close() }, nil
}

func withRelease(ctx context.Context, open openDataReturn, run func(organizationtenancy.DataReturnRelease) error) error {
	release, closeRelease, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeRelease()
	return run(release)
}

// recipientFlags binds who may retrieve with the token, how long, and where
// its secrets go.
func recipientFlags(cmd *cobra.Command, recipient *organizationtenancy.DataReturnRecipient, secrets *string) {
	f := cmd.Flags()
	f.StringVar(&recipient.Recipient, "recipient", "", "customer or target_provider")
	f.StringVar(&recipient.Name, "recipient-name", "", "Name of the recipient organisation, for the audit (not a person)")
	f.DurationVar(&recipient.ValidFor, "valid-for", 7*24*time.Hour, "Lifetime of the token, 1h to 720h")
	f.IntVar(&recipient.MaxDownloads, "max-downloads", 5, "Downloads the token allows, 1 to 20")
	f.StringVar(secrets, "secrets-out", "", "New file for token and archive key, in a private (0700) directory; never stdout")
	for _, name := range []string{"recipient", "recipient-name", "secrets-out"} {
		_ = cmd.MarkFlagRequired(name)
	}
}

func newTenantReturnRelease(open openDataReturn) *cobra.Command {
	var request organizationtenancy.DataReturnReleaseRequest
	var keepUntil, secrets string
	cmd := &cobra.Command{Use: "release", Short: "Encrypt a COMPLETE, checked package into the store and issue its first token", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			end, err := time.ParseInLocation("2006-01-02", keepUntil, time.UTC)
			if err != nil {
				return errors.New("data return: --keep-until is the last day of the retrieval period, YYYY-MM-DD")
			}
			request.KeepUntil = end.Add(24 * time.Hour)
			if err := checkSecretsTarget(secrets); err != nil {
				return err
			}
			return withRelease(cmd.Context(), open, func(release organizationtenancy.DataReturnRelease) error {
				issued, err := release.Release(cmd.Context(), request)
				if err != nil {
					return err
				}
				return reportIssued(cmd.OutOrStdout(), secrets, issued, true)
			})
		}}
	f := cmd.Flags()
	f.StringVar(&request.Package, "package", "", "Checked package directory with COMPLETE (output of tenant export)")
	f.Int64Var(&request.SchoolID, "school", 0, "School the package belongs to")
	f.Int64Var(&request.OrganizationID, "organization", 0, "Träger the package belongs to")
	f.Int64Var(&request.OperatorID, "operator", 0, "Active operator ID for the operator audit")
	f.StringVar(&keepUntil, "keep-until", "", "Last day of the retrieval period (YYYY-MM-DD); the archive is purged after it")
	cmd.MarkFlagsMutuallyExclusive("school", "organization")
	for _, name := range []string{"package", "operator", "keep-until"} {
		_ = cmd.MarkFlagRequired(name)
	}
	recipientFlags(cmd, &request.DataReturnRecipient, &secrets)
	return cmd
}

func newTenantReturnGrant(open openDataReturn) *cobra.Command {
	var request organizationtenancy.DataReturnGrantRequest
	var keyFile, secrets string
	cmd := &cobra.Command{Use: "grant", Short: "Issue another token for a released archive; the archive key comes from --archive-key-file or stdin", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := checkSecretsTarget(secrets); err != nil {
				return err
			}
			key, err := readArchiveKey(cmd.InOrStdin(), keyFile)
			if err != nil {
				return err
			}
			request.ArchiveKey = key
			return withRelease(cmd.Context(), open, func(release organizationtenancy.DataReturnRelease) error {
				issued, err := release.Grant(cmd.Context(), request)
				if err != nil {
					return err
				}
				return reportIssued(cmd.OutOrStdout(), secrets, issued, false)
			})
		}}
	cmd.Flags().StringVar(&request.ArchiveName, "archive", "", "Archive name the release printed")
	cmd.Flags().Int64Var(&request.OperatorID, "operator", 0, "Active operator ID for the operator audit")
	cmd.Flags().StringVar(&keyFile, "archive-key-file", "", "Secrets file of the release (or a file holding only the key); else stdin")
	_ = cmd.MarkFlagRequired("archive")
	_ = cmd.MarkFlagRequired("operator")
	recipientFlags(cmd, &request.DataReturnRecipient, &secrets)
	return cmd
}

func newTenantReturnRevoke(open openDataReturn) *cobra.Command {
	var request organizationtenancy.DataReturnRevokeRequest
	cmd := &cobra.Command{Use: "revoke", Short: "End one token (--grant), every token of an archive (--archive) or every token (--all, after a restore)", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withRelease(cmd.Context(), open, func(release organizationtenancy.DataReturnRelease) error {
				count, err := release.Revoke(cmd.Context(), request)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]int{"revoked": count})
			})
		}}
	cmd.Flags().Int64Var(&request.GrantID, "grant", 0, "Grant ID")
	cmd.Flags().StringVar(&request.ArchiveName, "archive", "", "Archive name")
	cmd.Flags().BoolVar(&request.All, "all", false, "Every token that still has an archive")
	cmd.Flags().Int64Var(&request.OperatorID, "operator", 0, "Active operator ID for the operator audit")
	cmd.MarkFlagsMutuallyExclusive("grant", "archive", "all")
	cmd.MarkFlagsOneRequired("grant", "archive", "all")
	_ = cmd.MarkFlagRequired("operator")
	return cmd
}

func newTenantReturnPurge(open openDataReturn) *cobra.Command {
	var operatorID int64
	cmd := &cobra.Command{Use: "purge", Short: "Remove archives past their retrieval period or fully revoked, files no grant names, old evidence", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withRelease(cmd.Context(), open, func(release organizationtenancy.DataReturnRelease) error {
				purged, err := release.Purge(cmd.Context(), operatorID)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(purged)
			})
		}}
	cmd.Flags().Int64Var(&operatorID, "operator", 0, "Active operator ID for the operator audit")
	_ = cmd.MarkFlagRequired("operator")
	return cmd
}

func newTenantReturnList(open openDataReturn) *cobra.Command {
	var operatorID int64
	cmd := &cobra.Command{Use: "list", Short: "List every grant without tokens or keys", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withRelease(cmd.Context(), open, func(release organizationtenancy.DataReturnRelease) error {
				grants, err := release.Grants(cmd.Context(), operatorID)
				if err != nil {
					return err
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(grants)
			})
		}}
	cmd.Flags().Int64Var(&operatorID, "operator", 0, "Active operator ID")
	_ = cmd.MarkFlagRequired("operator")
	return cmd
}

// checkSecretsTarget refuses a secrets file that would not stay private:
// standard output, an existing file, or a directory others can read.
func checkSecretsTarget(path string) error {
	clean := filepath.Clean(path)
	if path == "" || path == "-" || strings.HasPrefix(clean, "/dev/") || strings.HasPrefix(clean, "/proc/") {
		return errors.New("data return: --secrets-out must name a new file, never standard output")
	}
	if _, err := os.Lstat(clean); err == nil {
		return errors.New("data return: --secrets-out exists; name a new file")
	}
	dir, err := os.Stat(filepath.Dir(clean))
	if err != nil || !dir.IsDir() || dir.Mode().Perm()&0o077 != 0 {
		return errors.New("data return: the directory of --secrets-out must exist and be private (0700)")
	}
	return nil
}

// issuedSecrets is the content of the secrets file. Hand the token to the
// recipient over a confirmed channel; keep the archive key in the protected
// order until the retrieval period ends.
type issuedSecrets struct {
	GrantID     int64     `json:"grant_id"`
	Archive     string    `json:"archive"`
	ExpiresAt   time.Time `json:"expires_at"`
	Token       string    `json:"token"`
	ArchiveKey  string    `json:"archive_key,omitempty"`
	Instruction string    `json:"instruction"`
}

// reportIssued writes the secrets to a new 0600 file and only what is no
// secret to stdout.
func reportIssued(stdout io.Writer, path string, issued organizationtenancy.DataReturnIssued, withKey bool) error {
	secrets := issuedSecrets{GrantID: issued.GrantID, Archive: issued.ArchiveName, ExpiresAt: issued.ExpiresAt, Token: issued.Token,
		Instruction: "Token only to the named recipient over a confirmed channel; archive key only into the protected order. Delete this file afterwards."}
	if withKey {
		secrets.ArchiveKey = securityruntime.EncodeDataReturnArchiveKey(issued.ArchiveKey)
	}
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Clean(path), append(data, '\n')); err != nil {
		return fmt.Errorf("data return: grant %d stored, but its secrets file failed; revoke it with --grant %d and issue a new one: %w",
			issued.GrantID, issued.GrantID, err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]any{"grant_id": issued.GrantID, "archive": issued.ArchiveName, "archive_sha256": issued.ArchiveSHA256,
		"archive_bytes": issued.ArchiveBytes, "expires_at": issued.ExpiresAt, "keep_until": issued.KeepUntil, "secrets_file": path})
}

func writePrivateFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	if closeErr := file.Close(); writeErr == nil && syncErr == nil {
		return closeErr
	}
	return errors.Join(writeErr, syncErr)
}

// readArchiveKey reads the archive key from a secrets file of a release
// (its archive_key) or a file holding only the key, else from stdin.
func readArchiveKey(stdin io.Reader, path string) ([]byte, error) {
	var raw string
	if path != "" {
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, errors.New("data return: --archive-key-file unreadable")
		}
		var secrets issuedSecrets
		if json.Unmarshal(data, &secrets) == nil && secrets.ArchiveKey != "" {
			raw = secrets.ArchiveKey
		} else {
			raw = string(data)
		}
	} else {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, errors.New("data return: archive key unreadable on stdin")
		}
		raw = line
	}
	return securityruntime.DecodeDataReturnArchiveKey(raw)
}
