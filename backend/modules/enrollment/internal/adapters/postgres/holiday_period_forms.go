package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The versions of a holiday period's form (#5198). A version is written once
// and never changed; the request role may only read and add. Every
// statement runs in the Träger transaction of the Träger it names.

type holidayPeriodFormRow struct {
	Version   int             `bun:"version"`
	Fields    json.RawMessage `bun:"fields,type:jsonb"`
	CreatedAt time.Time       `bun:"created_at"`
}

func (row holidayPeriodFormRow) form() (enrollment.HolidayPeriodForm, error) {
	fields := []enrollment.HolidayFormField{}
	if err := json.Unmarshal(row.Fields, &fields); err != nil {
		return enrollment.HolidayPeriodForm{}, fmt.Errorf("enrollment postgres: decode holiday form version %d: %w", row.Version, err)
	}
	return enrollment.HolidayPeriodForm{Version: row.Version, Fields: fields, CreatedAt: row.CreatedAt}, nil
}

const (
	currentHolidayPeriodFormQuery = `SELECT version, fields, created_at FROM enrollment.holiday_period_forms
		WHERE organization_id = ? AND holiday_period_id = ?
		ORDER BY version DESC LIMIT 1`
	holidayPeriodFormVersionsQuery = `SELECT version, fields, created_at FROM enrollment.holiday_period_forms
		WHERE organization_id = ? AND holiday_period_id = ? AND version = ANY(?::int[])
		ORDER BY version`
	// insertHolidayPeriodFormQuery adds the next version. The caller holds
	// the period's row, so two saves of one period run one after the other;
	// the unique key on the version is the backstop.
	insertHolidayPeriodFormQuery = `INSERT INTO enrollment.holiday_period_forms
			(organization_id, holiday_period_id, version, fields, created_by)
		SELECT ?, ?, COALESCE(MAX(version), 0) + 1, ?::jsonb, ?
		FROM enrollment.holiday_period_forms
		WHERE organization_id = ? AND holiday_period_id = ?
		RETURNING version, fields, created_at`
)

// CurrentHolidayPeriodForm reads the newest version of a period's form.
func (s *HolidayPeriodStore) CurrentHolidayPeriodForm(ctx context.Context, organizationID, periodID int64) (enrollment.HolidayPeriodForm, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayPeriodForm{}, false, err
	}
	var rows []holidayPeriodFormRow
	if err := db.NewRaw(currentHolidayPeriodFormQuery, organizationID, periodID).Scan(ctx, &rows); err != nil {
		return enrollment.HolidayPeriodForm{}, false, fmt.Errorf("enrollment postgres: read holiday form: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayPeriodForm{}, false, nil
	}
	form, err := rows[0].form()
	return form, err == nil, err
}

// HolidayPeriodForms reads the named versions of a period's form, by
// version.
func (s *HolidayPeriodStore) HolidayPeriodForms(ctx context.Context, organizationID, periodID int64, versions []int) (map[int]enrollment.HolidayPeriodForm, error) {
	forms := map[int]enrollment.HolidayPeriodForm{}
	if len(versions) == 0 {
		return forms, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayPeriodFormRow
	if err := db.NewRaw(holidayPeriodFormVersionsQuery, organizationID, periodID, pgdialect.Array(versions)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday form versions: %w", err)
	}
	for _, row := range rows {
		form, err := row.form()
		if err != nil {
			return nil, err
		}
		forms[form.Version] = form
	}
	return forms, nil
}

// InsertHolidayPeriodForm adds the next version of a period's form. A zero
// createdBy leaves the author empty, as for the template of a new period.
func (s *HolidayPeriodStore) InsertHolidayPeriodForm(ctx context.Context, organizationID, periodID, createdBy int64, fields []enrollment.HolidayFormField) (enrollment.HolidayPeriodForm, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayPeriodForm{}, err
	}
	if fields == nil {
		fields = []enrollment.HolidayFormField{}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return enrollment.HolidayPeriodForm{}, fmt.Errorf("enrollment postgres: encode holiday form: %w", err)
	}
	var author *int64
	if createdBy > 0 {
		author = &createdBy
	}
	var row holidayPeriodFormRow
	if err := db.NewRaw(insertHolidayPeriodFormQuery, organizationID, periodID, string(encoded), author,
		organizationID, periodID).Scan(ctx, &row); err != nil {
		return enrollment.HolidayPeriodForm{}, fmt.Errorf("enrollment postgres: insert holiday form version: %w", err)
	}
	return row.form()
}
