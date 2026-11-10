package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/spf13/cobra"
)

// The operator side of the retrieval of a return package (Art. 30 Abs. 2
// Data Act, ADR 0203): release a checked package for HTTPS retrieval, issue
// further tokens, revoke them and purge the store. Every action is in the
// operator audit; tokens and keys are printed once and stored nowhere.

func newTenantReturnCommand() *cobra.Command {
	parent := &cobra.Command{Use: "return", Short: "Release a checked return package for HTTPS retrieval (Data Act)"}
	parent.AddCommand(newTenantReturnRelease(), newTenantReturnGrant(), newTenantReturnRevoke(), newTenantReturnPurge(), newTenantReturnList())
	return parent
}

// recipientFlags binds who may retrieve with the token and how long.
func recipientFlags(cmd *cobra.Command, recipient *organizationtenancy.DataReturnRecipient) {
	f := cmd.Flags()
	f.StringVar(&recipient.Recipient, "recipient", "", "customer or target_provider")
	f.StringVar(&recipient.Name, "recipient-name", "", "Name of the recipient organisation, for the audit (not a person)")
	f.DurationVar(&recipient.ValidFor, "valid-for", 7*24*time.Hour, "Lifetime of the token, 1h to 720h")
	f.IntVar(&recipient.MaxDownloads, "max-downloads", 5, "Downloads the token allows, 1 to 20")
	_ = cmd.MarkFlagRequired("recipient")
	_ = cmd.MarkFlagRequired("recipient-name")
}

func newTenantReturnRelease() *cobra.Command {
	var request organizationtenancy.DataReturnReleaseRequest
	var keepUntil string
	cmd := &cobra.Command{Use: "release", Short: "Encrypt a COMPLETE, checked package into the store and issue its first token", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			end, err := time.ParseInLocation("2006-01-02", keepUntil, time.UTC)
			if err != nil {
				return errors.New("data return: --keep-until is the last day of the retrieval period, YYYY-MM-DD")
			}
			request.KeepUntil = end.Add(24 * time.Hour)
			return withDataReturnRelease(cmd.Context(), func(release organizationtenancy.DataReturnRelease) error {
				issued, err := release.Release(cmd.Context(), request)
				if err != nil {
					return err
				}
				return printIssued(cmd.OutOrStdout(), issued, true)
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
	recipientFlags(cmd, &request.DataReturnRecipient)
	return cmd
}

func newTenantReturnGrant() *cobra.Command {
	var request organizationtenancy.DataReturnGrantRequest
	cmd := &cobra.Command{Use: "grant", Short: "Issue another token for a released archive; reads its archive key from stdin", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return errors.New("data return: archive key unreadable on stdin")
			}
			request.ArchiveKey, err = securityruntime.DecodeDataReturnArchiveKey(line)
			if err != nil {
				return err
			}
			return withDataReturnRelease(cmd.Context(), func(release organizationtenancy.DataReturnRelease) error {
				issued, err := release.Grant(cmd.Context(), request)
				if err != nil {
					return err
				}
				return printIssued(cmd.OutOrStdout(), issued, false)
			})
		}}
	cmd.Flags().StringVar(&request.ArchiveName, "archive", "", "Archive name the release printed")
	cmd.Flags().Int64Var(&request.OperatorID, "operator", 0, "Active operator ID for the operator audit")
	_ = cmd.MarkFlagRequired("archive")
	_ = cmd.MarkFlagRequired("operator")
	recipientFlags(cmd, &request.DataReturnRecipient)
	return cmd
}

func newTenantReturnRevoke() *cobra.Command {
	var request organizationtenancy.DataReturnRevokeRequest
	cmd := &cobra.Command{Use: "revoke", Short: "End one token (--grant) or every token of an archive (--archive) at once", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withDataReturnRelease(cmd.Context(), func(release organizationtenancy.DataReturnRelease) error {
				count, err := release.Revoke(cmd.Context(), request)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]int{"revoked": count})
			})
		}}
	cmd.Flags().Int64Var(&request.GrantID, "grant", 0, "Grant ID")
	cmd.Flags().StringVar(&request.ArchiveName, "archive", "", "Archive name")
	cmd.Flags().Int64Var(&request.OperatorID, "operator", 0, "Active operator ID for the operator audit")
	cmd.MarkFlagsMutuallyExclusive("grant", "archive")
	_ = cmd.MarkFlagRequired("operator")
	return cmd
}

func newTenantReturnPurge() *cobra.Command {
	var operatorID int64
	cmd := &cobra.Command{Use: "purge", Short: "Remove archives past their retrieval period or fully revoked; drop old evidence", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withDataReturnRelease(cmd.Context(), func(release organizationtenancy.DataReturnRelease) error {
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

func newTenantReturnList() *cobra.Command {
	var operatorID int64
	cmd := &cobra.Command{Use: "list", Short: "List every grant without tokens or keys", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withDataReturnRelease(cmd.Context(), func(release organizationtenancy.DataReturnRelease) error {
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

func withDataReturnRelease(ctx context.Context, run func(organizationtenancy.DataReturnRelease) error) error {
	db, err := transactionCompose.OpenDatabase()
	if err != nil {
		return errors.New("data return: database connection unavailable")
	}
	defer func() { _ = db.Close() }()
	audit, err := newDataReturnAudit(db)
	if err != nil {
		return err
	}
	release, err := organizationCompose.NewDataReturnRelease(db.Pool(), owners.NewDataReturnCrypto(), audit, organizationtenancy.DataReturnStoreDir)
	if err != nil {
		return err
	}
	return run(release)
}

// printIssued shows the secrets once. The token goes to the recipient over
// a confirmed channel; the archive key stays in the protected order.
func printIssued(w io.Writer, issued organizationtenancy.DataReturnIssued, withKey bool) error {
	out := map[string]any{"grant_id": issued.GrantID, "archive": issued.ArchiveName, "archive_sha256": issued.ArchiveSHA256,
		"archive_bytes": issued.ArchiveBytes, "expires_at": issued.ExpiresAt, "keep_until": issued.KeepUntil, "token": issued.Token}
	if withKey {
		out["archive_key"] = securityruntime.EncodeDataReturnArchiveKey(issued.ArchiveKey)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "Token and archive key are shown only now. Hand the token over a confirmed channel; keep the key in the protected order until the retrieval period ends.")
	return err
}

// dataReturnAudit writes each data return action into the operator audit,
// like the export (#6326): an unknown or inactive operator changes nothing.
type dataReturnAudit struct {
	log       owners.OperatorAuditEntries
	operators identityaccess.OperatorDirectory
}

func newDataReturnAudit(db transactionCompose.Database) (dataReturnAudit, error) {
	operators, err := identityaccessCompose.NewOperatorDirectory(db.Pool())
	if err != nil {
		return dataReturnAudit{}, err
	}
	return dataReturnAudit{log: owners.NewOperatorAuditLog(db), operators: operators}, nil
}

func (a dataReturnAudit) BeginDataReturn(ctx context.Context, operatorID int64) error {
	if operatorID <= 0 {
		return errors.New("data return audit: --operator is required")
	}
	operator, err := a.operators.FindOperator(ctx, operatorID)
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return errors.New("data return audit: unknown operator")
	}
	if err != nil {
		return fmt.Errorf("data return audit: operator unavailable: %w", err)
	}
	if !operator.Active {
		return errors.New("data return audit: operator is not active")
	}
	return nil
}

func (a dataReturnAudit) RecordDataReturn(ctx context.Context, record organizationtenancy.DataReturnAuditRecord) error {
	changes, err := json.Marshal(record)
	if err != nil {
		return err
	}
	resourceType, resourceID := "data_return", (*int64)(nil)
	if record.Package != "" {
		resourceType, resourceID = record.Package, &record.SubjectID
	}
	return a.write(ctx, record.OperatorID, "data_return_"+record.Action, resourceType, resourceID, changes)
}

func (a dataReturnAudit) write(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, changes []byte) error {
	return a.log.Create(ctx, &auditModels.OperatorAuditEntry{OperatorID: operatorID, Action: action,
		ResourceType: resourceType, ResourceID: resourceID, Changes: changes})
}
