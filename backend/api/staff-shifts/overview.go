package staffshifts

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
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

type WeeklySummaryResponse struct {
	StaffID        int64  `json:"staff_id"`
	WeekStart      string `json:"week_start"`
	PlannedMinutes int    `json:"planned_minutes"`
	TargetMinutes  *int   `json:"target_minutes"`
	DeltaMinutes   *int   `json:"delta_minutes"`
}

type OverviewResponse struct {
	From            string                  `json:"from"`
	To              string                  `json:"to"`
	DienstplanInUse bool                    `json:"dienstplan_in_use"`
	UsedWeeks       []string                `json:"dienstplan_used_weeks"`
	Staff           []OverviewStaffResponse `json:"staff"`
	Shifts          []ShiftResponse         `json:"shifts"`
	Assignments     []AssignmentResponse    `json:"assignments"`
	WeeklySummaries []WeeklySummaryResponse `json:"weekly_summaries"`
}

func toOverviewResponse(overview workforce.StaffScheduleOverview) OverviewResponse {
	usedWeeks := make([]string, 0, len(overview.UsedWeeks))
	usedWeeks = append(usedWeeks, overview.UsedWeeks...)
	staff := make([]OverviewStaffResponse, 0, len(overview.Staff))
	for _, member := range overview.Staff {
		staff = append(staff, OverviewStaffResponse{
			ID:        member.ID,
			FirstName: member.FirstName,
			LastName:  member.LastName,
		})
	}

	assignments := make([]AssignmentResponse, 0, len(overview.Assignments))
	for _, assignment := range overview.Assignments {
		intervals := make([]CoverageIntervalResponse, 0, len(assignment.UncoveredIntervals))
		for _, interval := range assignment.UncoveredIntervals {
			intervals = append(intervals, CoverageIntervalResponse{
				StartTime: FormatWallClock(interval.StartTime),
				EndTime:   FormatWallClock(interval.EndTime),
			})
		}
		assignments = append(assignments, AssignmentResponse{
			InstanceID:         assignment.InstanceID,
			StaffID:            assignment.StaffID,
			Date:               assignment.Date,
			StartTime:          FormatWallClock(assignment.StartTime),
			EndTime:            FormatWallClock(assignment.EndTime),
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

	weeklySummaries := make([]WeeklySummaryResponse, 0, len(overview.WeeklySummaries))
	for _, summary := range overview.WeeklySummaries {
		weeklySummaries = append(weeklySummaries, WeeklySummaryResponse{
			StaffID:        summary.StaffID,
			WeekStart:      summary.WeekStart,
			PlannedMinutes: summary.PlannedMinutes,
			TargetMinutes:  summary.TargetMinutes,
			DeltaMinutes:   summary.DeltaMinutes,
		})
	}

	return OverviewResponse{
		From:            overview.From,
		To:              overview.To,
		DienstplanInUse: overview.DienstplanInUse,
		UsedWeeks:       usedWeeks,
		Staff:           staff,
		Shifts:          ToShiftResponses(overview.Shifts),
		Assignments:     assignments,
		WeeklySummaries: weeklySummaries,
	}
}

func (rs *Resource) overview(w http.ResponseWriter, r *http.Request) {
	from, to, ok := rs.parseDateRange(w, r)
	if !ok {
		return
	}
	overview, err := rs.planning.Overview(r.Context(), from, to)
	if err != nil {
		if errors.Is(err, workforce.ErrInvalidStaffShift) || errors.Is(err, workforce.ErrStaffShiftRangeTooLarge) {
			rs.renderError(w, r, err)
			return
		}
		rs.runtime.Failure(w, r, FailureInternal, &ClientMessageError{Message: staffScheduleOverviewLoadErrorMessage, Cause: err})
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, toOverviewResponse(overview), "Staff schedule overview retrieved")
}
