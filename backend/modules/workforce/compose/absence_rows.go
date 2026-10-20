package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

// AbsenceRequestRecords is the enriched request-list port supplied by the root.
type AbsenceRequestRecords interface {
	AbsenceRecords
	ListRequests(context.Context, AbsenceRequestFilter) ([]*AbsenceRequestRow, error)
}

type (
	StaffAbsence         = timetracking.StaffAbsence
	AbsenceRecords       = timetracking.AbsenceRecords
	AbsenceAuditRecords  = timetracking.AbsenceAuditRecords
	AbsenceRequestFilter = timetracking.AbsenceRequestFilter
	AbsenceRequestRow    = timetracking.AbsenceRequestRow
)

// AbsenceRequestRows maps the native list result for root-owned person enrichment.
func AbsenceRequestRows(values []workforce.StaffAbsence, err error) ([]*AbsenceRequestRow, error) {
	if err != nil {
		if errors.Is(err, workforce.ErrInvalidStaffAbsence) {
			return nil, errors.New(err.Error())
		}
		return nil, timetracking.AbsenceRequestReadError(err)
	}
	rows := make([]*AbsenceRequestRow, 0, len(values))
	for _, value := range values {
		rows = append(rows, &AbsenceRequestRow{StaffAbsence: timetracking.AbsenceRecord(value)})
	}
	return rows, nil
}
