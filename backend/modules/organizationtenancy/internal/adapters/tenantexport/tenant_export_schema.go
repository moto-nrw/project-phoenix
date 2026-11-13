package tenantexport

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

type exportSchemaColumn struct {
	TableName string
	Name      string
	Type      string
	UDT       string
	Nullable  string
}

type exportSchemaRelation struct {
	TableName    string
	Column       string
	TargetTable  string
	TargetColumn string
}

// Schema metadata belongs to this export's read-only snapshot, not to the
// exporter or database pool. A later export must observe schema changes.
type exportSchema struct {
	columns   map[string][]exportSchemaColumn
	relations map[string][]exportSchemaRelation
	// primaryKeys names a row of a table; a Träger package names its
	// document files by it.
	primaryKeys map[string][]string
}

type exportSchemaKey struct {
	TableName string
	Column    string
}

func loadExportSchema(ctx context.Context, tx bun.Tx, catalog organizationtenancy.DataLifecycleCatalog) (*exportSchema, error) {
	var names []string
	for _, table := range catalog.Tables {
		if table.Export == "include" {
			names = append(names, table.Name)
		}
	}
	var columns []exportSchemaColumn
	err := tx.NewRaw(`SELECT table_schema || '.' || table_name AS table_name,
  column_name AS name, data_type AS type, udt_name AS udt, is_nullable AS nullable
  FROM information_schema.columns
  WHERE table_schema || '.' || table_name IN (?)
  ORDER BY table_schema, table_name, ordinal_position`, bun.List(names)).Scan(ctx, &columns)
	if err != nil {
		return nil, err
	}
	var relations []exportSchemaRelation
	err = tx.NewRaw(`SELECT src_ns.nspname || '.' || src.relname AS table_name,
  a.attname AS column, ns.nspname || '.' || c.relname AS target_table, b.attname AS target_column
  FROM pg_catalog.pg_constraint fk
  JOIN pg_catalog.pg_class src ON src.oid = fk.conrelid
  JOIN pg_catalog.pg_namespace src_ns ON src_ns.oid = src.relnamespace
  JOIN pg_catalog.pg_class c ON c.oid = fk.confrelid
  JOIN pg_catalog.pg_namespace ns ON ns.oid = c.relnamespace
  CROSS JOIN LATERAL generate_subscripts(fk.conkey, 1) AS n(i)
  JOIN pg_catalog.pg_attribute a ON a.attrelid = fk.conrelid AND a.attnum = fk.conkey[n.i]
  JOIN pg_catalog.pg_attribute b ON b.attrelid = fk.confrelid AND b.attnum = fk.confkey[n.i]
  WHERE fk.contype = 'f' AND src_ns.nspname || '.' || src.relname IN (?)
  ORDER BY src_ns.nspname, src.relname, a.attname, ns.nspname, c.relname`, bun.List(names)).Scan(ctx, &relations)
	if err != nil {
		return nil, err
	}
	var keys []exportSchemaKey
	err = tx.NewRaw(`SELECT ns.nspname || '.' || c.relname AS table_name, a.attname AS column
  FROM pg_catalog.pg_index i
  JOIN pg_catalog.pg_class c ON c.oid = i.indrelid
  JOIN pg_catalog.pg_namespace ns ON ns.oid = c.relnamespace
  CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, n)
  JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum
  WHERE i.indisprimary AND ns.nspname || '.' || c.relname IN (?)
  ORDER BY ns.nspname, c.relname, k.n`, bun.List(names)).Scan(ctx, &keys)
	if err != nil {
		return nil, err
	}
	schema := &exportSchema{columns: make(map[string][]exportSchemaColumn), relations: make(map[string][]exportSchemaRelation),
		primaryKeys: make(map[string][]string)}
	for _, key := range keys {
		schema.primaryKeys[key.TableName] = append(schema.primaryKeys[key.TableName], key.Column)
	}
	for _, column := range columns {
		schema.columns[column.TableName] = append(schema.columns[column.TableName], column)
	}
	for _, relation := range relations {
		schema.relations[relation.TableName] = append(schema.relations[relation.TableName], relation)
	}
	return schema, nil
}
