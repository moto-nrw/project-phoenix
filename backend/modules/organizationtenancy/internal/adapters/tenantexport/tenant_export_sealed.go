package tenantexport

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"

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

// projectedColumnsOf answers the container columns the export projects, in
// a fixed order.
func projectedColumnsOf(table organizationtenancy.LifecycleTable) []string {
	columns := make([]string, 0, len(table.ProjectedColumns))
	for container := range table.ProjectedColumns {
		columns = append(columns, container)
	}
	slices.Sort(columns)
	return columns
}

// sealedBindingColumns selects the value of each sealed binding (#6412) as
// a correlated lookup in the same snapshot. Every name is checked against
// the schema; a binding that names an absent column or collides with a
// column of the table fails the export.
func (r *exportRun) sealedBindingColumns(table organizationtenancy.LifecycleTable) ([]string, error) {
	keys := make([]string, 0, len(table.SealedBindings))
	for key := range table.SealedBindings {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	own := r.schema.columns[table.Name]
	var selects []string
	for _, key := range keys {
		binding := table.SealedBindings[key]
		_, collides := schemaColumn(own, key)
		_, known := schemaColumn(r.schema.columns[binding.Table], binding.Column)
		if collides || !known || len(binding.Match) == 0 || len(table.OpenSealedColumns) == 0 {
			return nil, exportTableError(table.Name)
		}
		locals := make([]string, 0, len(binding.Match))
		for local := range binding.Match {
			locals = append(locals, local)
		}
		slices.Sort(locals)
		conditions := make([]string, 0, len(locals))
		for _, local := range locals {
			_, localKnown := schemaColumn(own, local)
			_, targetKnown := schemaColumn(r.schema.columns[binding.Table], binding.Match[local])
			if !localKnown || !targetKnown {
				return nil, exportTableError(table.Name)
			}
			conditions = append(conditions, "b."+quoteExportIdentifier(binding.Match[local])+" = t."+quoteExportIdentifier(local))
		}
		selects = append(selects, "(SELECT b."+quoteExportIdentifier(binding.Column)+" FROM "+quoteExportTable(binding.Table)+
			" AS b WHERE "+strings.Join(conditions, " AND ")+") AS "+quoteExportIdentifier(key))
	}
	return selects, nil
}

// ownerModelColumns describes the columns the owners of the table write in
// place of its sealed and container columns.
func ownerModelColumns(table organizationtenancy.LifecycleTable, columns []exportSchemaColumn) ([]organizationtenancy.ExportColumn, error) {
	var owned []organizationtenancy.ExportColumn
	for _, sealed := range sealedColumnsOf(table) {
		col, ok := schemaColumn(columns, sealed)
		if !ok {
			return nil, exportTableError(table.Name)
		}
		owned = append(owned, organizationtenancy.ExportColumn{Name: table.OpenSealedColumns[sealed], Type: "text", UDT: "text", Nullable: col.Nullable == "YES"})
	}
	for _, container := range projectedColumnsOf(table) {
		col, ok := schemaColumn(columns, container)
		if !ok {
			return nil, exportTableError(table.Name)
		}
		// A withheld value is null, whatever the container allows.
		owned = append(owned, organizationtenancy.ExportColumn{Name: table.ProjectedColumns[container], Type: col.Type, UDT: col.UDT, Nullable: true})
	}
	return owned, nil
}

func schemaColumn(columns []exportSchemaColumn, name string) (exportSchemaColumn, bool) {
	for _, col := range columns {
		if col.Name == name {
			return col, true
		}
	}
	return exportSchemaColumn{}, false
}

// applyOwnerColumns replaces each sealed column of one exported row with its
// plaintext column (#3838, ADR 0192) and each container column with its
// reviewed projection (ADR 0193). The ciphertext is useless without the key
// of the deployment, so the return carries the value as entered; a container
// never leaves raw.
func (r *exportRun) applyOwnerColumns(table organizationtenancy.LifecycleTable, data []byte) ([]byte, error) {
	if len(table.OpenSealedColumns) == 0 && len(table.ProjectedColumns) == 0 {
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
	// A binding only lets the owner open the row; it is no export column.
	for key := range table.SealedBindings {
		delete(row, key)
	}
	for _, container := range projectedColumnsOf(table) {
		raw := row[container]
		delete(row, container)
		value, err := r.projectValue(table.Name, container, row, raw)
		if err != nil {
			return nil, err
		}
		row[table.ProjectedColumns[container]] = value
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

// projectValue lets the owner of the table keep the customer data of one
// container value and drop its secrets. A container without an owner fails
// the export; it is never copied raw.
func (r *exportRun) projectValue(table, column string, row map[string]json.RawMessage, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("null"), nil
	}
	if r.projected == nil {
		return nil, errors.New("tenant export: " + table + " holds container values; bind the owner that projects them")
	}
	value, err := r.projected.ProjectExportField(table, column, row, raw)
	if err != nil || !json.Valid(value) {
		return nil, errors.New("tenant export: cannot project a value of " + table)
	}
	return value, nil
}
