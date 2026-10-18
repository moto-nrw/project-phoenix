package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The bindings below connect the class-day slot lists (#1565) to the session
// permissions, the caller's staff record and Document Rendering.

// ClassDaySlotListStaff resolves the caller's staff member for the slot
// lists' read verdict. found is false, without an error, for a person who is
// no staff member; an account without a person is an error.
type ClassDaySlotListStaff interface {
	CurrentStaffIDOfPerson(ctx context.Context) (staffID int64, found bool, err error)
}

// ClassDaySlotListAccess binds the slot lists' read verdict (Security
// Runtime's SlotListStudentAccess) to the session permissions and the
// caller's staff record. Staff may be nil; the verdict then rests on the
// permissions alone.
type ClassDaySlotListAccess struct{ Staff ClassDaySlotListStaff }

// CanReadStudents collapses only the legitimate restrictive condition (no
// staff record) to "no"; operational failures propagate.
func (a ClassDaySlotListAccess) CanReadStudents(ctx context.Context) (bool, error) {
	return securityruntime.SlotListStudentAccess(sessiontoken.PermissionsFromCtx(ctx), func() (bool, error) {
		if a.Staff == nil {
			return false, nil
		}
		_, found, err := a.Staff.CurrentStaffIDOfPerson(ctx)
		if err != nil {
			return false, fmt.Errorf("resolve current staff: %w", err)
		}
		return found, nil
	})
}

// ClassDaySlotListRenderer renders the slot list exports with Document
// Rendering.
type ClassDaySlotListRenderer struct{ Renderer lists.Renderer }

var classDayExportColumns = map[classday.ExportColumnID]lists.ColumnID{
	classday.ExportColumnName:           lists.ColumnName,
	classday.ExportColumnSchoolClass:    lists.ColumnSchoolClass,
	classday.ExportColumnGroup:          lists.ColumnGroup,
	classday.ExportColumnSlot:           lists.ColumnSlot,
	classday.ExportColumnPlannedPickup:  lists.ColumnPlannedPickup,
	classday.ExportColumnPresenceStatus: lists.ColumnPresenceStatus,
}

var classDayExportFormats = map[classday.Format]lists.Format{
	classday.FormatPDF:  lists.FormatPDF,
	classday.FormatXLSX: lists.FormatXLSX,
}

// Render translates the slot list document into a list document and renders
// it in the requested format.
func (r ClassDaySlotListRenderer) Render(doc classday.ExportDocument, format classday.Format, filenameBase string) (classday.File, error) {
	listFormat, ok := classDayExportFormats[format]
	if !ok {
		return classday.File{}, fmt.Errorf("unsupported slot list format %q", format)
	}
	columns := make([]lists.Column, 0, len(doc.Columns))
	for _, column := range doc.Columns {
		id, err := classDayExportColumn(column.ID)
		if err != nil {
			return classday.File{}, err
		}
		columns = append(columns, lists.Column{ID: id, Label: column.Label})
	}
	rows := make([]lists.Row, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		var values map[lists.ColumnID]string
		if row.Values != nil {
			values = make(map[lists.ColumnID]string, len(row.Values))
		}
		for columnID, value := range row.Values {
			id, err := classDayExportColumn(columnID)
			if err != nil {
				return classday.File{}, err
			}
			values[id] = value
		}
		rows = append(rows, lists.Row{GroupTitle: row.GroupTitle, Values: values})
	}
	file, err := r.Renderer.Render(lists.Document{
		Title: doc.Title, Subtitle: doc.Subtitle, GeneratedAt: doc.GeneratedAt, Filters: doc.Filters,
		Columns: columns, Rows: rows, Footer: doc.Footer,
	}, listFormat, filenameBase)
	if err != nil {
		return classday.File{}, err
	}
	return classday.File{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func classDayExportColumn(id classday.ExportColumnID) (lists.ColumnID, error) {
	column, ok := classDayExportColumns[id]
	if !ok {
		return "", fmt.Errorf("unknown slot list column %q", id)
	}
	return column, nil
}
