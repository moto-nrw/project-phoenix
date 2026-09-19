package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type workSessionAudit struct {
	audit.WorkSessionEditRepository
}

// NewWorkSessionAudit projects audit storage onto the work-session contract.
func NewWorkSessionAudit(repo audit.WorkSessionEditRepository) timetracking.WorkSessionAudit {
	if repo == nil {
		return nil
	}
	return workSessionAudit{repo}
}

func (a workSessionAudit) CreateBatch(ctx context.Context, edits []*timetracking.WorkSessionEdit) error {
	var rows []*audit.WorkSessionEdit
	if edits != nil {
		rows = make([]*audit.WorkSessionEdit, len(edits))
	}
	for i, edit := range edits {
		if edit == nil {
			continue
		}
		row := &audit.WorkSessionEdit{ID: edit.ID, SessionID: edit.SessionID, StaffID: edit.StaffID, EditedBy: edit.EditedBy, FieldName: edit.FieldName, OldValue: edit.OldValue, NewValue: edit.NewValue, Notes: edit.Notes, CreatedAt: edit.CreatedAt}
		row.SetTenantID(edit.TenantID)
		rows[i] = row
	}
	err := a.WorkSessionEditRepository.CreateBatch(ctx, rows)
	for i, row := range rows {
		if row != nil && edits[i] != nil {
			*edits[i] = *projectWorkSessionEdit(row)
		}
	}
	return err
}

func (a workSessionAudit) GetBySessionID(ctx context.Context, id int64) ([]*timetracking.WorkSessionEdit, error) {
	rows, err := a.WorkSessionEditRepository.GetBySessionID(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*timetracking.WorkSessionEdit, len(rows))
	for i, row := range rows {
		result[i] = projectWorkSessionEdit(row)
	}
	return result, err
}

func projectWorkSessionEdit(row *audit.WorkSessionEdit) *timetracking.WorkSessionEdit {
	if row == nil {
		return nil
	}
	return &timetracking.WorkSessionEdit{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, StaffID: row.StaffID, EditedBy: row.EditedBy, FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue, Notes: row.Notes, CreatedAt: row.CreatedAt}
}
