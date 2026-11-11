package tenantexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

type tenantExporter struct {
	db           *bun.DB
	fingerprints organizationtenancy.ExportFingerprints
	sealed       organizationtenancy.TenantExportSealedFields
	projected    organizationtenancy.TenantExportProjectedFields
	sepaFiles    organizationtenancy.TenantExportSEPADebitFiles
	audit        organizationtenancy.TenantExportAudit
}

func NewTenantExporter(db *bun.DB, dependencies organizationtenancy.TenantExportDependencies) organizationtenancy.TenantExporter {
	return &tenantExporter{db: db, fingerprints: dependencies.Fingerprints, sealed: dependencies.Sealed, projected: dependencies.Projected,
		sepaFiles: dependencies.SEPADebitFiles, audit: dependencies.Audit}
}

type exportRun struct {
	tx           bun.Tx
	options      organizationtenancy.TenantExportOptions
	catalog      organizationtenancy.DataLifecycleCatalog
	manifest     organizationtenancy.ExportManifest
	schema       *exportSchema
	models       []organizationtenancy.ExportModelTable
	references   map[string]bool
	fingerprints organizationtenancy.ExportFingerprints
	sealed       organizationtenancy.TenantExportSealedFields
	projected    organizationtenancy.TenantExportProjectedFields
	person       *personSelection
}

func (e *tenantExporter) Export(ctx context.Context, options organizationtenancy.TenantExportOptions) error {
	options, err := validateExportOptions(options)
	if err != nil {
		return err
	}
	return e.exportAudited(ctx, options)
}

// run exports into the new output directory. step names what failed,
// created whether this run created the output directory. complete records
// the package in the operator audit before its COMPLETE marker.
func (e *tenantExporter) run(ctx context.Context, options organizationtenancy.TenantExportOptions,
	complete func(organizationtenancy.TenantExportRecord) error,
) (step string, created bool, err error) {
	run, err := e.start(ctx, options)
	if err != nil {
		return "snapshot", false, err
	}
	defer func() { _ = run.tx.Rollback() }()
	if err = os.Mkdir(options.Output, 0o700); err != nil {
		return "filesystem", false, errors.New("tenant export: cannot create new private output directory")
	}
	defer func() {
		if err != nil {
			_ = os.Remove(filepath.Join(options.Output, "COMPLETE"))
		}
	}()
	if err = run.exportSnapshot(ctx); err != nil {
		return "snapshot", true, err
	}
	if options.OrganizationID > 0 {
		if err = run.writeDocumentFiles(ctx); err != nil {
			return "document_files", true, err
		}
		if err = run.writeSEPADebitFiles(ctx, e.sepaFiles); err != nil {
			return "sepa_debit_files", true, err
		}
	}
	step, err = run.finish(complete)
	return step, true, err
}

// exportSnapshot writes everything the read-only snapshot holds and commits it.
func (r *exportRun) exportSnapshot(ctx context.Context) error {
	if err := r.exportTables(ctx); err != nil {
		return err
	}
	if r.options.Person != "" {
		if err := r.selectPerson(ctx); err != nil {
			return err
		}
	}
	if err := r.exportFiles(ctx); err != nil {
		return err
	}
	if r.person != nil {
		if err := r.writePersonReview(); err != nil {
			return err
		}
	}
	if err := r.writeJSON("model.json", r.models); err != nil {
		return err
	}
	if err := r.tx.Commit(); err != nil {
		return errors.New("tenant export: snapshot commit failed")
	}
	return nil
}

func validateExportOptions(options organizationtenancy.TenantExportOptions) (organizationtenancy.TenantExportOptions, error) {
	if options.Person != "" {
		if _, _, err := parseExportPerson(options.Person); err != nil {
			return options, err
		}
	}
	if (options.SchoolID > 0) == (options.OrganizationID > 0) || options.SchoolID < 0 || options.OrganizationID < 0 {
		return options, errors.New("tenant export: exactly one school or Träger is required")
	}
	if options.Person != "" && options.OrganizationID > 0 {
		return options, errors.New("tenant export: a person review needs the school that holds the person")
	}
	if options.Output == "" || options.Uploads == "" {
		return options, errors.New("tenant export: new output directory and uploads root are required")
	}
	uploads, err := filepath.EvalSymlinks(options.Uploads)
	if err != nil {
		return options, errors.New("tenant export: uploads root unavailable")
	}
	uploads, err = filepath.Abs(uploads)
	if err != nil {
		return options, errors.New("tenant export: invalid uploads root")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(options.Output))
	if err != nil {
		return options, errors.New("tenant export: output parent unavailable")
	}
	out, err := filepath.Abs(filepath.Join(parent, filepath.Base(options.Output)))
	if err != nil {
		return options, errors.New("tenant export: invalid output path")
	}
	if out == uploads || strings.HasPrefix(out, uploads+string(os.PathSeparator)) || strings.HasPrefix(uploads, out+string(os.PathSeparator)) {
		return options, errors.New("tenant export: output and uploads must be separate trees")
	}
	options.Output, options.Uploads = out, uploads
	return options, nil
}

func (e *tenantExporter) start(ctx context.Context, options organizationtenancy.TenantExportOptions) (*exportRun, error) {
	source, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	if err != nil {
		return nil, errors.New("tenant export: catalog unavailable")
	}
	r := &exportRun{options: options, references: make(map[string]bool), fingerprints: e.fingerprints, sealed: e.sealed, projected: e.projected}
	if err = json.Unmarshal(source, &r.catalog); err != nil {
		return nil, errors.New("tenant export: invalid catalog")
	}
	r.manifest = organizationtenancy.ExportManifest{FormatVersion: organizationtenancy.ExportFormatVersion, CatalogVersion: r.catalog.Version, CatalogSHA256: r.fingerprints.Bytes(source)}
	r.tx, err = e.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, errors.New("tenant export: cannot open read-only snapshot")
	}
	if err = r.prepare(ctx); err != nil {
		_ = r.tx.Rollback()
		return nil, err
	}
	return r, nil
}

func (r *exportRun) prepare(ctx context.Context) error {
	// The NOLOGIN read-only admin role can see every school. Explicit predicates
	// below, not HTTP middleware or an ambient tenant, define this export.
	if _, err := r.tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant_export"); err != nil {
		return errors.New("tenant export: administrative read role unavailable")
	}
	var safe bool
	if err := r.tx.NewRaw(`SELECT current_setting('transaction_read_only') = 'on'
		AND current_setting('transaction_isolation') = 'repeatable read'
		AND current_user = 'phoenix_tenant_export'`).Scan(ctx, &safe); err != nil || !safe {
		return errors.New("tenant export: unsafe snapshot configuration")
	}
	if err := r.requireSubject(ctx); err != nil {
		return err
	}
	var started time.Time
	if err := r.tx.NewRaw("SELECT transaction_timestamp()").Scan(ctx, &started); err != nil {
		return errors.New("tenant export: snapshot time unavailable")
	}
	r.manifest.ExportedAt = started.UTC().Format(time.RFC3339Nano)
	return nil
}

func (r *exportRun) exportTables(ctx context.Context) error {
	for _, table := range r.catalog.Tables {
		entry := organizationtenancy.ExportTable{Name: table.Name, Classification: table.Export, Reason: table.ExportReason, ExcludedColumns: table.ExcludeSecretColumns}
		if table.Export == "include" && r.inSchoolPackages(table) {
			entry.Classification, entry.Reason = "schoolPackage", "Held by a school; returned in the package of each school of the Träger."
		} else if table.Export == "include" {
			model, err := r.tableModel(ctx, table)
			if err != nil {
				return err
			}
			entry.Rows, err = r.exportTable(ctx, table, model)
			if err != nil {
				return err
			}
			r.models = append(r.models, model)
		} else if table.Export != "excludeSecret" && table.Export != "excludeInternal" {
			return errors.New("tenant export: unknown catalog classification")
		}
		r.manifest.Tables = append(r.manifest.Tables, entry)
	}
	return nil
}

func (r *exportRun) writeJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return errors.New("tenant export: cannot encode metadata")
	}
	return r.writeArtifact(name, append(data, '\n'))
}

func (r *exportRun) writeArtifact(name string, data []byte) error {
	if err := privateExportFile(filepath.Join(r.options.Output, name), data); err != nil {
		return err
	}
	r.manifest.Artifacts = append(r.manifest.Artifacts, organizationtenancy.ExportArtifact{Path: name, SHA256: r.fingerprints.Bytes(data), Bytes: int64(len(data))})
	return nil
}

// finish writes the manifest and, last, the COMPLETE marker. Every package
// is in the operator audit before it is complete: a run whose record fails
// can never be handed over (ADR 0187, #6326).
func (r *exportRun) finish(complete func(organizationtenancy.TenantExportRecord) error) (string, error) {
	data, err := json.MarshalIndent(r.manifest, "", "  ")
	if err != nil {
		return "manifest", errors.New("tenant export: cannot encode manifest")
	}
	data = append(data, '\n')
	if err := privateExportFile(filepath.Join(r.options.Output, "manifest.json"), data); err != nil {
		return "manifest", err
	}
	checksum := r.fingerprints.Bytes(data)
	if err := complete(r.auditRecord(checksum)); err != nil {
		return "audit", err
	}
	return "complete_marker", privateExportFile(filepath.Join(r.options.Output, "COMPLETE"), []byte(checksum+"\n"))
}

// syncFile makes an artifact durable before the export records it: a
// package is complete only once every file it names survives a crash.
var syncFile = (*os.File).Sync

func privateExportFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("tenant export: cannot create artifact")
	}
	_, writeErr := f.Write(data)
	syncErr := syncFile(f)
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return errors.New("tenant export: cannot persist artifact")
	}
	return nil
}

func exportTableError(table string) error { return fmt.Errorf("tenant export: table %s failed", table) }
