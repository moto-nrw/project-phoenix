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
}

func NewTenantExporter(db *bun.DB, fingerprints organizationtenancy.ExportFingerprints) organizationtenancy.TenantExporter {
	return &tenantExporter{db: db, fingerprints: fingerprints}
}

type exportRun struct {
	tx           bun.Tx
	options      organizationtenancy.TenantExportOptions
	catalog      organizationtenancy.DataLifecycleCatalog
	manifest     organizationtenancy.ExportManifest
	models       []organizationtenancy.ExportModelTable
	references   map[string]bool
	fingerprints organizationtenancy.ExportFingerprints
	person       *personSelection
}

func (e *tenantExporter) Export(ctx context.Context, options organizationtenancy.TenantExportOptions) (err error) {
	options, err = validateExportOptions(options)
	if err != nil {
		return err
	}
	run, err := e.start(ctx, options)
	if err != nil {
		return err
	}
	defer func() { _ = run.tx.Rollback() }()
	if err = os.Mkdir(options.Output, 0o700); err != nil {
		return errors.New("tenant export: cannot create new private output directory")
	}
	defer func() {
		if err != nil {
			_ = os.Remove(filepath.Join(options.Output, "COMPLETE"))
		}
	}()
	if err = run.exportTables(ctx); err != nil {
		return err
	}
	if options.Person != "" {
		if err = run.selectPerson(ctx); err != nil {
			return err
		}
	}
	if err = run.exportFiles(ctx); err != nil {
		return err
	}
	if run.person != nil {
		if err = run.writePersonReview(); err != nil {
			return err
		}
	}
	if err = run.writeJSON("model.json", run.models); err != nil {
		return err
	}
	if err = run.tx.Commit(); err != nil {
		return errors.New("tenant export: snapshot commit failed")
	}
	return run.finish()
}

func validateExportOptions(options organizationtenancy.TenantExportOptions) (organizationtenancy.TenantExportOptions, error) {
	if options.Person != "" {
		if _, _, err := parseExportPerson(options.Person); err != nil {
			return options, err
		}
	}
	if options.SchoolID <= 0 || options.Output == "" || options.Uploads == "" {
		return options, errors.New("tenant export: school, new output directory and uploads root are required")
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
	r := &exportRun{options: options, references: make(map[string]bool), fingerprints: e.fingerprints}
	if err = json.Unmarshal(source, &r.catalog); err != nil {
		return nil, errors.New("tenant export: invalid catalog")
	}
	r.manifest = organizationtenancy.ExportManifest{FormatVersion: 1, CatalogVersion: r.catalog.Version, CatalogSHA256: r.fingerprints.Bytes(source)}
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
	var exists bool
	if err := r.tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM platform.schools WHERE id = ?)`, r.options.SchoolID).Scan(ctx, &exists); err != nil || !exists {
		return errors.New("tenant export: school unavailable")
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
		if table.Export == "include" {
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

func (r *exportRun) finish() error {
	data, err := json.MarshalIndent(r.manifest, "", "  ")
	if err != nil {
		return errors.New("tenant export: cannot encode manifest")
	}
	data = append(data, '\n')
	if err := privateExportFile(filepath.Join(r.options.Output, "manifest.json"), data); err != nil {
		return err
	}
	return privateExportFile(filepath.Join(r.options.Output, "COMPLETE"), []byte(r.fingerprints.Bytes(data)+"\n"))
}

func privateExportFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("tenant export: cannot create artifact")
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return errors.New("tenant export: cannot persist artifact")
	}
	return nil
}

func exportTableError(table string) error { return fmt.Errorf("tenant export: table %s failed", table) }
