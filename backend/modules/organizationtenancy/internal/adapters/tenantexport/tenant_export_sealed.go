package tenantexport

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// sealedColumnsOf answers the field-sealed columns the export opens, in a
// fixed order so the select list and the model stay stable.
func sealedColumnsOf(table organizationtenancy.LifecycleTable) []string {
	columns := make([]string, 0, len(table.OpenSealedColumns))
	for sealed := range table.OpenSealedColumns {
		columns = append(columns, sealed)
	}
	slices.Sort(columns)
	return columns
}

func schemaColumn(columns []exportSchemaColumn, name string) (exportSchemaColumn, bool) {
	for _, col := range columns {
		if col.Name == name {
			return col, true
		}
	}
	return exportSchemaColumn{}, false
}

// openSealedColumns replaces each sealed column of one exported row with its
// plaintext column (#3838, ADR 0184). The ciphertext is useless without the
// key of the deployment, so the return carries the value as entered.
func (r *exportRun) openSealedColumns(table organizationtenancy.LifecycleTable, data []byte) ([]byte, error) {
	if len(table.OpenSealedColumns) == 0 {
		return data, nil
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, exportTableError(table.Name)
	}
	for _, column := range sealedColumnsOf(table) {
		raw := row[column]
		delete(row, column)
		value, err := r.openSealedValue(table.Name, column, row, raw)
		if err != nil {
			return nil, err
		}
		row[table.OpenSealedColumns[column]] = value
	}
	data, err := json.Marshal(row)
	if err != nil {
		return nil, exportTableError(table.Name)
	}
	return data, nil
}

// openSealedValue opens one hex-encoded sealed value. A value that does not
// open fails the export; it is never left out silently.
func (r *exportRun) openSealedValue(table, column string, row map[string]json.RawMessage, raw json.RawMessage) (json.RawMessage, error) {
	var encoded *string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, exportTableError(table)
	}
	if encoded == nil {
		return json.RawMessage("null"), nil
	}
	if r.sealed == nil {
		return nil, errors.New("tenant export: " + table + " holds sealed values; bind the key ring that opens them")
	}
	sealed, err := hex.DecodeString(*encoded)
	if err != nil {
		return nil, exportTableError(table)
	}
	value, err := r.sealed.OpenExportField(table, column, row, sealed)
	if err != nil {
		return nil, errors.New("tenant export: cannot open a sealed value of " + table)
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return nil, exportTableError(table)
	}
	return plain, nil
}
