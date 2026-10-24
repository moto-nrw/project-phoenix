package tenantexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func (r *exportRun) tableModel(ctx context.Context, table organizationtenancy.LifecycleTable) (organizationtenancy.ExportModelTable, error) {
	model := organizationtenancy.ExportModelTable{Name: table.Name}
	parts := strings.Split(table.Name, ".")
	if len(parts) != 2 {
		return model, exportTableError(table.Name)
	}
	var columns []struct {
		Name     string
		Type     string
		UDT      string
		Nullable string
	}
	err := r.tx.NewRaw(`SELECT column_name AS name, data_type AS type, udt_name AS udt, is_nullable AS nullable
		FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`, parts[0], parts[1]).Scan(ctx, &columns)
	if err != nil || len(columns) == 0 {
		return model, exportTableError(table.Name)
	}
	for _, col := range columns {
		if _, excluded := table.ExcludeSecretColumns[col.Name]; !excluded {
			model.Columns = append(model.Columns, organizationtenancy.ExportColumn{Name: col.Name, Type: col.Type, UDT: col.UDT, Nullable: col.Nullable == "YES"})
		}
	}
	var relations []struct {
		Column       string
		TargetTable  string
		TargetColumn string
	}
	err = r.tx.NewRaw(`SELECT a.attname AS column, ns.nspname || '.' || c.relname AS target_table, b.attname AS target_column
		FROM pg_catalog.pg_constraint fk
		JOIN pg_catalog.pg_class c ON c.oid = fk.confrelid
		JOIN pg_catalog.pg_namespace ns ON ns.oid = c.relnamespace
		CROSS JOIN LATERAL generate_subscripts(fk.conkey, 1) AS n(i)
		JOIN pg_catalog.pg_attribute a ON a.attrelid = fk.conrelid AND a.attnum = fk.conkey[n.i]
		JOIN pg_catalog.pg_attribute b ON b.attrelid = fk.confrelid AND b.attnum = fk.confkey[n.i]
		WHERE fk.contype = 'f' AND fk.conrelid = ?::regclass ORDER BY a.attname, ns.nspname, c.relname`, table.Name).Scan(ctx, &relations)
	if err != nil {
		return model, exportTableError(table.Name)
	}
	for _, rel := range relations {
		if _, excluded := table.ExcludeSecretColumns[rel.Column]; !excluded {
			model.Relationships = append(model.Relationships, organizationtenancy.ExportRelationship{Column: rel.Column, TargetTable: rel.TargetTable, TargetColumn: rel.TargetColumn})
		}
	}
	return model, nil
}

func (r *exportRun) exportTable(ctx context.Context, table organizationtenancy.LifecycleTable, model organizationtenancy.ExportModelTable) (int64, error) {
	where, err := r.scope(table, "t", 0)
	if err != nil {
		return 0, err
	}
	columns := make([]string, 0, len(model.Columns))
	for _, col := range model.Columns {
		columns = append(columns, "t."+quoteExportIdentifier(col.Name))
	}
	if len(columns) == 0 {
		return 0, exportTableError(table.Name)
	}
	order, err := exportOrder(table, model)
	if err != nil {
		return 0, err
	}
	query := "SELECT row_to_json(export_row) FROM (SELECT " + strings.Join(columns, ", ") + " FROM " + quoteExportTable(table.Name) + " AS t WHERE " + where + " ORDER BY " + order + ") AS export_row"
	rows, err := r.tx.QueryContext(ctx, query, r.options.SchoolID)
	if err != nil {
		return 0, exportTableError(table.Name)
	}
	defer func() { _ = rows.Close() }()
	name := table.Name + ".jsonl"
	f, err := os.OpenFile(filepath.Join(r.options.Output, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, errors.New("tenant export: cannot create table artifact")
	}
	defer func() { _ = f.Close() }()
	count, err := r.streamExportRows(rows, f, table.Name)
	if err != nil {
		return 0, err
	}
	if f.Sync() != nil || f.Close() != nil {
		return 0, errors.New("tenant export: cannot persist table artifact")
	}
	if err := r.recordArtifact(name); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *exportRun) streamExportRows(rows *sql.Rows, w io.Writer, table string) (int64, error) {
	var count int64
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return 0, exportTableError(table)
		}
		if err := r.collectReferences(table, data); err != nil {
			return 0, err
		}
		_, err := w.Write(append(data, '\n'))
		if err != nil {
			return 0, errors.New("tenant export: cannot write table artifact")
		}
		count++
	}
	if rows.Err() != nil {
		return 0, exportTableError(table)
	}
	return count, nil
}

func (r *exportRun) scope(table organizationtenancy.LifecycleTable, alias string, depth int) (string, error) {
	if depth > len(r.catalog.Tables) {
		return "", errors.New("tenant export: cyclic scope path")
	}
	switch table.Scope {
	case "tenant":
		return alias + ".tenant_id = ?0", nil
	case "school", "tenantReference":
		return alias + "." + quoteExportIdentifier(table.ScopeColumn) + " = ?0", nil
	case "organization":
		return alias + "." + quoteExportIdentifier(table.ScopeColumn) + " IN (SELECT organization_id FROM platform.schools WHERE id = ?0)", nil
	case "tenantChild":
		return r.childScope(table, alias, depth)
	case "account":
		return accountExportScope(table.Name, alias)
	default:
		return "", exportTableError(table.Name)
	}
}

func (r *exportRun) childScope(table organizationtenancy.LifecycleTable, alias string, depth int) (string, error) {
	if table.Parent == nil {
		return "", exportTableError(table.Name)
	}
	for _, parent := range r.catalog.Tables {
		if parent.Name != table.Parent.Table {
			continue
		}
		p := alias + "p"
		where, err := r.scope(parent, p, depth+1)
		if err != nil {
			return "", err
		}
		return "EXISTS (SELECT 1 FROM " + quoteExportTable(parent.Name) + " AS " + p + " WHERE " + p + "." + quoteExportIdentifier(table.Parent.ParentColumn) + " = " + alias + "." + quoteExportIdentifier(table.Parent.Column) + " AND " + where + ")", nil
	}
	return "", exportTableError(table.Name)
}

// Shared identity metadata is included only for accounts needed by this
// school, including former/pending memberships. Never read B's memberships.
const exportAccounts = `(SELECT account_id FROM auth.account_tenants WHERE tenant_id = ?0
	UNION SELECT account_id FROM users.persons WHERE tenant_id = ?0 AND account_id IS NOT NULL
	UNION SELECT account_id FROM users.guardian_profiles WHERE tenant_id = ?0 AND account_id IS NOT NULL
	UNION SELECT account_id FROM users.profiles WHERE tenant_id = ?0 AND account_id IS NOT NULL
	UNION SELECT guardian_account_id FROM enrollment.requests WHERE tenant_id = ?0 AND guardian_account_id IS NOT NULL)`

func accountExportScope(table, alias string) (string, error) {
	switch table {
	case "auth.accounts":
		return alias + ".id IN " + exportAccounts, nil
	case "auth.mfa_credentials", "auth.passkey_credentials", "auth.password_reset_tokens":
		return alias + ".account_id IN " + exportAccounts, nil
	case "auth.password_reset_rate_limits":
		return alias + ".email IN (SELECT email FROM auth.accounts WHERE id IN " + exportAccounts + ")", nil
	case "platform.announcement_views":
		return alias + ".user_id IN " + exportAccounts, nil
	default:
		return "", exportTableError(table)
	}
}

func quoteExportIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
func quoteExportTable(name string) string {
	parts := strings.Split(name, ".")
	for i := range parts {
		parts[i] = quoteExportIdentifier(parts[i])
	}
	return strings.Join(parts, ".")
}

func (r *exportRun) collectReferences(table string, data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return exportTableError(table)
	}
	if err := r.collectStoredOriginal(table, fields); err != nil {
		return err
	}
	if err := r.collectLegalOriginals(table, fields); err != nil {
		return err
	}
	for column, raw := range fields {
		if !exportURLReferenceField(table, column) {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) == nil && strings.HasPrefix(value, "/uploads/") {
			key := strings.TrimPrefix(value, "/uploads/")
			if !r.allowedObject(key, table == "auth.accounts" || table == "users.profiles") {
				return errors.New("tenant export: invalid or foreign object reference")
			}
			r.references[key] = true
		}
	}
	return nil
}

// A user-authored name or message that happens to look like an upload URL is
// not authority to return an object, especially from the global avatar store.
func exportURLReferenceField(table, column string) bool {
	if column == "avatar" {
		return table == "auth.accounts" || table == "users.profiles"
	}
	return table == "users.student_profiles" && column == "photo_path"
}

// Stored document names are not URLs. Completed byte-deletion/cleanup rows
// remain useful metadata, but must not require an original already removed.
func (r *exportRun) collectStoredOriginal(table string, fields map[string]json.RawMessage) error {
	kinds := map[string]string{
		"documents.files": "files", "documents.announcement_attachments": "announcement-attachments",
		"users.student_documents": "student-documents", "users.staff_documents": "staff-documents",
	}
	kind, known := kinds[table]
	if !known {
		return nil
	}
	for _, completion := range []string{"file_deleted_at", "cleaned_at"} {
		if value, ok := fields[completion]; ok && string(value) != "null" {
			return nil
		}
	}
	var name string
	if value, ok := fields["filename_stored"]; !ok || json.Unmarshal(value, &name) != nil || name == "" {
		return nil
	}
	key := kind + "/" + strconv.FormatInt(r.options.SchoolID, 10) + "/" + name
	if strings.ContainsAny(name, `/\`) || !r.allowedObject(key, false) {
		return errors.New("tenant export: invalid stored original reference")
	}
	r.references[key] = true
	return nil
}

// exportOrder is the ORDER BY of a table artifact: the physical order, or the
// catalog's columns where the physical order would link rows the table keeps
// apart (ExportOrder). Every named column must be one the artifact exports.
func exportOrder(table organizationtenancy.LifecycleTable, model organizationtenancy.ExportModelTable) (string, error) {
	if len(table.ExportOrder) == 0 {
		return "t.ctid", nil
	}
	exported := make(map[string]bool, len(model.Columns))
	for _, col := range model.Columns {
		exported[col.Name] = true
	}
	parts := make([]string, 0, len(table.ExportOrder))
	for _, name := range table.ExportOrder {
		if !exported[name] {
			return "", exportTableError(table.Name)
		}
		parts = append(parts, "t."+quoteExportIdentifier(name))
	}
	return strings.Join(parts, ", "), nil
}
