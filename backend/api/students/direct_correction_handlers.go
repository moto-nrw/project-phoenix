package students

import (
	"strconv"
	"time"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// DirectCorrectionResponse is one admin correction to a child's bookings as
// the central history renders it (#2436). It deliberately carries no status
// and no decision fields: a correction is not a request, it has no open state
// and nothing was decided about it.
type DirectCorrectionResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	// ChangedAt is when the office applied the correction.
	ChangedAt time.Time `json:"changed_at"`
	// ChangedByName is the actor snapshot taken at write time, "Unbekannt"
	// when the account is gone.
	ChangedByName string                        `json:"changed_by_name"`
	Reason        string                        `json:"reason"`
	Diff          []OfferingRequestDiffResponse `json:"diff"`
}

func toDirectCorrectionResponse(item *enrollmentService.DirectCorrectionItem) DirectCorrectionResponse {
	row := item.Adjustment
	diff := make([]OfferingRequestDiffResponse, 0, len(item.Diff))
	for _, entry := range item.Diff {
		diff = append(diff, OfferingRequestDiffResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Label,
			Old:        germanOfferingDiffLabel(entry.OldState, entry.OldDays),
			New:        germanOfferingDiffLabel(entry.NewState, entry.NewDays),
		})
	}
	return DirectCorrectionResponse{
		ID:            strconv.FormatInt(row.ID, 10),
		StudentID:     strconv.FormatInt(row.StudentID, 10),
		StudentName:   item.StudentName,
		ChangedAt:     row.ChangedAt,
		ChangedByName: item.ActorName,
		Reason:        row.Reason,
		Diff:          diff,
	}
}

func directCorrectionRow(item *enrollmentService.DirectCorrectionItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeDirectCorrection,
		sortTime:    item.Adjustment.ChangedAt,
		id:          item.Adjustment.ID,
		studentID:   item.Adjustment.StudentID,
		studentName: item.StudentName,
		// A correction has no request status; the date-range filter treats the
		// moment it was applied as its "decided at".
		decidedAt: item.Adjustment.ChangedAt,
		data:      toDirectCorrectionResponse(item),
	}
}
