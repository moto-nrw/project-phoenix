package staffshifts

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

const staffScheduleOverviewLoadErrorMessage = "staff schedule overview could not be loaded"

type OverviewStaffResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type CoverageIntervalResponse struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type AssignmentResponse struct {
	InstanceID         int64                      `json:"instance_id"`
	StaffID            int64                      `json:"staff_id"`
	Date               string                     `json:"date"`
	StartTime          string                     `json:"start_time"`
	EndTime            string                     `json:"end_time"`
	ActivityTitle      string                     `json:"activity_title"`
	RoomID             int64                      `json:"room_id"`
	RoomName           string                     `json:"room_name"`
	Status             string                     `json:"status"`
	IsAbsent           bool                       `json:"is_absent"`
	IsSubstitute       bool                       `json:"is_substitute"`
	AbsenceReason      *string                    `json:"absence_reason"`
	CoverageStatus     string                     `json:"coverage_status"`
	CoverageReason     *string                    `json:"coverage_reason"`
	UncoveredIntervals []CoverageIntervalResponse `json:"uncovered_intervals"`
}

type OverviewResponse struct {
	From            string                  `json:"from"`
	To              string                  `json:"to"`
	DienstplanInUse bool                    `json:"dienstplan_in_use"`
	Staff           []OverviewStaffResponse `json:"staff"`
	Shifts          []ShiftResponse         `json:"shifts"`
	Assignments     []AssignmentResponse    `json:"assignments"`
}

func toOverviewResponse(overview *scheduleSvc.StaffScheduleOverview) OverviewResponse {
	staff := make([]OverviewStaffResponse, 0, len(overview.Staff))
	for _, member := range overview.Staff {
		if member == nil || member.Person == nil {
			continue
		}
		staff = append(staff, OverviewStaffResponse{
			ID:        member.ID,
			FirstName: member.Person.FirstName,
			LastName:  member.Person.LastName,
		})
	}

	assignments := make([]AssignmentResponse, 0, len(overview.Assignments))
	for _, assignment := range overview.Assignments {
		intervals := make([]CoverageIntervalResponse, 0, len(assignment.UncoveredIntervals))
		for _, interval := range assignment.UncoveredIntervals {
			intervals = append(intervals, CoverageIntervalResponse{
				StartTime: timezone.WallClock(interval.StartTime).Format("15:04"),
				EndTime:   timezone.WallClock(interval.EndTime).Format("15:04"),
			})
		}
		assignments = append(assignments, AssignmentResponse{
			InstanceID:         assignment.InstanceID,
			StaffID:            assignment.StaffID,
			Date:               assignment.Date.String(),
			StartTime:          timezone.WallClock(assignment.StartTime).Format("15:04"),
			EndTime:            timezone.WallClock(assignment.EndTime).Format("15:04"),
			ActivityTitle:      assignment.ActivityTitle,
			RoomID:             assignment.RoomID,
			RoomName:           assignment.RoomName,
			Status:             assignment.Status,
			IsAbsent:           assignment.IsAbsent,
			IsSubstitute:       assignment.IsSubstitute,
			AbsenceReason:      assignment.AbsenceReason,
			CoverageStatus:     assignment.CoverageStatus,
			CoverageReason:     assignment.CoverageReason,
			UncoveredIntervals: intervals,
		})
	}

	return OverviewResponse{
		From:            overview.From.String(),
		To:              overview.To.String(),
		DienstplanInUse: overview.DienstplanInUse,
		Staff:           staff,
		Shifts:          ToShiftResponses(overview.Shifts),
		Assignments:     assignments,
	}
}

func (rs *Resource) overview(w http.ResponseWriter, r *http.Request) {
	if rs.Overview == nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			staffScheduleOverviewLoadErrorMessage,
			errors.New("staff schedule overview not wired"),
		))
		return
	}
	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}
	overview, err := rs.Overview.GetOverview(r.Context(), from, to)
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrShiftInvalid) || errors.Is(err, scheduleSvc.ErrShiftRangeTooLarge) {
			renderServiceError(w, r, err)
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap(staffScheduleOverviewLoadErrorMessage, err))
		return
	}
	common.Respond(w, r, http.StatusOK, toOverviewResponse(overview), "Staff schedule overview retrieved")
}
