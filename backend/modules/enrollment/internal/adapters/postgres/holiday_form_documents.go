package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (s *HolidayPeriodStore) InsertHolidayFormDocument(ctx context.Context, org, period, actor int64, filename string, content []byte) (enrollment.HolidayFormDocument, error) {
	db, err := traegerDB(ctx, org)
	if err != nil {
		return enrollment.HolidayFormDocument{}, err
	}
	var id string
	err = db.NewRaw(`INSERT INTO enrollment.holiday_form_documents
		(organization_id, holiday_period_id, filename, content, size, created_by)
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id`, org, period, filename, content, len(content), actor).Scan(ctx, &id)
	if err != nil {
		return enrollment.HolidayFormDocument{}, fmt.Errorf("enrollment postgres: insert holiday PDF: %w", err)
	}
	return enrollment.HolidayFormDocument{ID: id, Filename: filename, Size: len(content)}, nil
}

func (s *HolidayPeriodStore) ReadHolidayFormDocument(ctx context.Context, org, period int64, id string, content bool) (enrollment.HolidayFormDocument, bool, error) {
	db, err := traegerDB(ctx, org)
	if err != nil {
		return enrollment.HolidayFormDocument{}, false, err
	}
	var rows []struct {
		ID       string `bun:"id"`
		Filename string `bun:"filename"`
		Content  []byte `bun:"content"`
		Size     int    `bun:"size"`
	}
	err = db.NewRaw(`SELECT id, filename, CASE WHEN ? THEN content END AS content, size FROM enrollment.holiday_form_documents
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?`, content, org, period, id).Scan(ctx, &rows)
	if err != nil {
		return enrollment.HolidayFormDocument{}, false, fmt.Errorf("enrollment postgres: read holiday PDF: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayFormDocument{}, false, nil
	}
	r := rows[0]
	return enrollment.HolidayFormDocument{ID: r.ID, Filename: r.Filename, Content: r.Content, Size: r.Size}, true, nil
}

func (s *HolidayPeriodStore) DeleteHolidayFormDocument(ctx context.Context, org, period int64, id string) (bool, error) {
	db, err := traegerDB(ctx, org)
	if err != nil {
		return false, err
	}
	result, err := db.NewRaw(`DELETE FROM enrollment.holiday_form_documents AS doc
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?
		AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_period_forms AS form,
			jsonb_array_elements(form.legal_blocks) AS block,
			jsonb_array_elements(COALESCE(block->'attachments', '[]'::jsonb)) AS attachment
			WHERE form.organization_id = doc.organization_id AND form.holiday_period_id = doc.holiday_period_id
			AND attachment->>'document_id' = doc.id::text)`, org, period, id).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: delete holiday PDF: %w", err)
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
