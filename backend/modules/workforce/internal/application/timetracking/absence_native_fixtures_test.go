package timetracking_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

func recordAbsenceAudit(ctx context.Context, records timetracking.AbsenceAuditRecords, audit *timetracking.StaffAbsenceAudit) error {
	if err := audit.Validate(); err != nil {
		return err
	}
	value, err := records.RecordStaffAbsenceAudit(ctx, workforce.StaffAbsenceAudit{TenantID: audit.TenantID, AbsenceID: audit.AbsenceID, FromStatus: audit.FromStatus, ToStatus: audit.ToStatus, ActorID: audit.ActorID, Note: audit.Note, ChangedAt: audit.ChangedAt, TypeChange: (*workforce.AbsenceTypeChange)(audit.TypeChange)})
	if err != nil {
		return err
	}
	audit.ID = value.ID
	audit.TenantID = value.TenantID
	audit.ChangedAt = value.ChangedAt
	return nil
}
func vacationOpeningRowsByStaff(ctx context.Context, records timetracking.VacationOpeningRecords, ids []int64, year int) (map[int64]workforce.StaffVacationOpening, error) {
	result := make(map[int64]workforce.StaffVacationOpening)
	if len(ids) == 0 {
		return result, nil
	}
	values, err := records.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffIDs: ids, Year: year})
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		result[value.StaffID] = value
	}
	return result, nil
}
