package services

import (
	"context"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type workSessionAudit struct {
	audit.WorkSessionEditRepository
}

// NewWorkSessionAudit projects audit storage onto the work-session contract.
func NewWorkSessionAudit(repo audit.WorkSessionEditRepository) workforceCompose.WorkSessionAudit {
	if repo == nil {
		return nil
	}
	return workSessionAudit{repo}
}

func (a workSessionAudit) CreateBatch(ctx context.Context, edits []*workforceCompose.WorkSessionEdit) error {
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

func (a workSessionAudit) GetBySessionID(ctx context.Context, id int64) ([]*workforceCompose.WorkSessionEdit, error) {
	rows, err := a.WorkSessionEditRepository.GetBySessionID(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*workforceCompose.WorkSessionEdit, len(rows))
	for i, row := range rows {
		result[i] = projectWorkSessionEdit(row)
	}
	return result, err
}

func projectWorkSessionEdit(row *audit.WorkSessionEdit) *workforceCompose.WorkSessionEdit {
	if row == nil {
		return nil
	}
	return &workforceCompose.WorkSessionEdit{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, StaffID: row.StaffID, EditedBy: row.EditedBy, FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue, Notes: row.Notes, CreatedAt: row.CreatedAt}
}
