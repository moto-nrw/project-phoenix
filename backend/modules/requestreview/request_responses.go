package requestreview

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

func ToMasterDataChangeRequestResponse(item *masterdatarequests.ReviewItem) MasterDataChangeRequestResponse {
	r := item.Request
	return MasterDataChangeRequestResponse{
		ID:         strconv.FormatInt(r.ID, 10),
		StudentID:  strconv.FormatInt(r.StudentID, 10),
		FirstName:  item.FirstName,
		LastName:   item.LastName,
		Target:     r.Target,
		FieldKey:   r.FieldKey,
		OldValue:   r.OldValue,
		NewValue:   r.NewValue,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt,
		ReviewedAt: r.ReviewedAt,
	}
}

func ToMasterDataHistoryResponse(item *masterdatarequests.HistoryItem) MasterDataChangeRequestHistoryResponse {
	return MasterDataChangeRequestHistoryResponse{
		MasterDataChangeRequestResponse: ToMasterDataChangeRequestResponse(&masterdatarequests.ReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		ReviewReason:  item.Request.ReviewReason,
	}
}

func ToStaffExcusedRequestResponse(item *excusedrequests.ReviewItem) StaffExcusedRequestResponse {
	r := item.Request
	dates := make([]string, 0, len(r.Dates))
	for _, d := range r.Dates {
		dates = append(dates, d.String())
	}
	return StaffExcusedRequestResponse{
		ID:            strconv.FormatInt(r.ID, 10),
		StudentID:     strconv.FormatInt(r.StudentID, 10),
		FirstName:     item.FirstName,
		LastName:      item.LastName,
		AbsenceStatus: r.AbsenceStatus,
		Status:        r.Status,
		Dates:         dates,
		Note:          r.Note,
		Reason:        r.DecisionReason,
		CreatedAt:     r.CreatedAt,
		ReviewedAt:    r.ReviewedAt,
	}
}

func ToStaffExcusedHistoryResponse(item *excusedrequests.HistoryItem) StaffExcusedRequestHistoryResponse {
	return StaffExcusedRequestHistoryResponse{
		StaffExcusedRequestResponse: ToStaffExcusedRequestResponse(&excusedrequests.ReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
	}
}
