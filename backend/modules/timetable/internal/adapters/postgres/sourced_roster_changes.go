package postgres

import (
	"context"
	"encoding/json"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) SaveSourcedRosterChanges(ctx context.Context, changes timetable.SourcedRosterChanges) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	total := domain.OperationStats{}
	if len(changes.Delete) > 0 {
		stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("activities.student_enrollments").Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(changes.Delete)), "delete sourced roster rows")
		total.Add(stats)
		if err != nil {
			return total, err
		}
	}
	if len(changes.Ends) > 0 {
		stats, err := capSourcedRosterRows(ctx, db, tenantID, changes.Ends)
		total.Add(stats)
		if err != nil {
			return total, err
		}
	}
	if len(changes.Create) > 0 {
		rows := make([]studentEnrollmentRow, 0, len(changes.Create))
		for _, input := range changes.Create {
			row := studentEnrollmentRow{TenantID: tenantID}
			applyStudentEnrollmentFields(&row, input)
			rows = append(rows, row)
		}
		stats, err := execMeasuredWrite(ctx, db.NewInsert().Model(&rows).ModelTableExpr("activities.student_enrollments").Returning("NULL"), "create sourced roster rows")
		total.Add(stats)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (s *Store) UpdateGroupOfferingSources(ctx context.Context, changes map[int64]timetable.OfferingSourceInput) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	type sourceRow struct {
		ID        int64
		Offerings []int64
		Grades    []int
		Classes   []string
	}
	rows := make([]sourceRow, 0, len(changes))
	for id, in := range changes {
		rows = append(rows, sourceRow{id, in.CareOfferingIDs, in.GradeLevels, in.SchoolClasses})
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewRaw(`UPDATE activities.groups AS g SET source_care_offering_ids = NULLIF(x."Offerings", 'null'::jsonb),
  source_grade_levels = NULLIF(x."Grades", 'null'::jsonb), source_school_classes = NULLIF(x."Classes", 'null'::jsonb), updated_at = NOW()
  FROM jsonb_to_recordset(?::jsonb) AS x("ID" bigint, "Offerings" jsonb, "Grades" jsonb, "Classes" jsonb)
  WHERE g.tenant_id = ? AND g.id = x."ID"`, string(payload), tenantID), "update group offering sources")
}

func (s *Store) SaveSourcedInstanceStudents(ctx context.Context, creates []timetable.InstanceStudentInput, deletes []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	total := domain.OperationStats{}
	if len(deletes) > 0 {
		stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("schedule.instance_students").Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(deletes)), "delete sourced instance students")
		total.Add(stats)
		if err != nil {
			return total, err
		}
	}
	if len(creates) > 0 {
		rows := make([]instanceStudentRow, 0, len(creates))
		for _, in := range creates {
			rows = append(rows, instanceStudentRow{TenantID: tenantID, InstanceID: in.InstanceID, StudentID: in.StudentID, RoomID: in.RoomID})
		}
		stats, err := execMeasuredWrite(ctx, db.NewInsert().Model(&rows).ModelTableExpr("schedule.instance_students").Returning("NULL"), "create sourced instance students")
		total.Add(stats)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func capSourcedRosterRows(ctx context.Context, db bun.IDB, tenantID int64, endsByID map[int64]string) (domain.OperationStats, error) {
	type endRow struct {
		ID    int64
		Until string
	}
	ends := make([]endRow, 0, len(endsByID))
	for id, end := range endsByID {
		ends = append(ends, endRow{id, end})
	}
	payload, err := json.Marshal(ends)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewRaw(`UPDATE activities.student_enrollments AS e SET valid_until = x."Until"::date
   FROM jsonb_to_recordset(?::jsonb) AS x("ID" bigint, "Until" text) WHERE e.tenant_id = ? AND e.id = x."ID"`, string(payload), tenantID), "cap sourced roster rows")
}
