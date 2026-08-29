package statistics

import (
	"strconv"

	statisticsService "github.com/moto-nrw/project-phoenix/services/statistics"
)

// int64 IDs travel as strings on the wire (frontend convention).

type studentResponse struct {
	StudentID       string   `json:"student_id"`
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	SchoolClass     string   `json:"school_class"`
	GroupID         *string  `json:"group_id,omitempty"`
	GroupName       string   `json:"group_name"`
	PresentDays     int      `json:"present_days"`
	SickDays        int      `json:"sick_days"`
	ExcusedDays     int      `json:"excused_days"`
	UnexplainedDays int      `json:"unexplained_days"`
	AttendanceRate  *float64 `json:"attendance_rate"`
}

type groupResponse struct {
	GroupID         string   `json:"group_id"`
	Name            string   `json:"name"`
	StudentCount    int      `json:"student_count"`
	PresentDays     int      `json:"present_days"`
	SickDays        int      `json:"sick_days"`
	ExcusedDays     int      `json:"excused_days"`
	UnexplainedDays int      `json:"unexplained_days"`
	AttendanceRate  *float64 `json:"attendance_rate"`
}

type roomResponse struct {
	RoomID                 string   `json:"room_id"`
	Name                   string   `json:"name"`
	Capacity               *int     `json:"capacity,omitempty"`
	DaysUsed               int      `json:"days_used"`
	DistinctStudents       int      `json:"distinct_students"`
	StudentMinutes         int      `json:"student_minutes"`
	PeakOccupancy          int      `json:"peak_occupancy"`
	PeakUtilizationPercent *float64 `json:"peak_utilization_percent"`
}

type excludedDaysResponse struct {
	Total          int `json:"total"`
	PublicHolidays int `json:"public_holidays"`
	ClosingDays    int `json:"closing_days"`
	HolidayPeriods int `json:"holiday_periods"`
}

type reportResponse struct {
	From         string               `json:"from"`
	To           string               `json:"to"`
	CareDays     int                  `json:"care_days"`
	ExcludedDays excludedDaysResponse `json:"excluded_days"`
	Totals       groupResponse        `json:"totals"`
	Students     []studentResponse    `json:"students"`
	Groups       []groupResponse      `json:"groups"`
	Rooms        []roomResponse       `json:"rooms"`
	RoomDataDays int                  `json:"room_data_days"`
	RoomDataFrom string               `json:"room_data_from"`
}

func toReportResponse(report *statisticsService.Report) reportResponse {
	out := reportResponse{
		From:     report.From.String(),
		To:       report.To.String(),
		CareDays: report.CareDays,
		ExcludedDays: excludedDaysResponse{
			Total:          report.ExcludedDays.Total,
			PublicHolidays: report.ExcludedDays.PublicHolidays,
			ClosingDays:    report.ExcludedDays.ClosingDays,
			HolidayPeriods: report.ExcludedDays.HolidayPeriods,
		},
		Totals:       toGroupResponse(report.Totals),
		Students:     make([]studentResponse, 0, len(report.Students)),
		Groups:       make([]groupResponse, 0, len(report.Groups)),
		Rooms:        make([]roomResponse, 0, len(report.Rooms)),
		RoomDataDays: report.RoomDataDays,
		RoomDataFrom: report.RoomDataFrom.String(),
	}
	for _, st := range report.Students {
		row := studentResponse{
			StudentID:       strconv.FormatInt(st.StudentID, 10),
			FirstName:       st.FirstName,
			LastName:        st.LastName,
			SchoolClass:     st.SchoolClass,
			GroupName:       st.GroupName,
			PresentDays:     st.PresentDays,
			SickDays:        st.SickDays,
			ExcusedDays:     st.ExcusedDays,
			UnexplainedDays: st.UnexplainedDays,
			AttendanceRate:  st.AttendanceRate,
		}
		if st.GroupID != nil {
			id := strconv.FormatInt(*st.GroupID, 10)
			row.GroupID = &id
		}
		out.Students = append(out.Students, row)
	}
	for _, g := range report.Groups {
		out.Groups = append(out.Groups, toGroupResponse(g))
	}
	for _, room := range report.Rooms {
		out.Rooms = append(out.Rooms, roomResponse{
			RoomID:                 strconv.FormatInt(room.RoomID, 10),
			Name:                   room.Name,
			Capacity:               room.Capacity,
			DaysUsed:               room.DaysUsed,
			DistinctStudents:       room.DistinctStudents,
			StudentMinutes:         room.StudentMinutes,
			PeakOccupancy:          room.PeakOccupancy,
			PeakUtilizationPercent: room.PeakUtilizationPercent,
		})
	}
	return out
}

func toGroupResponse(g statisticsService.GroupRow) groupResponse {
	return groupResponse{
		GroupID:         strconv.FormatInt(g.GroupID, 10),
		Name:            g.Name,
		StudentCount:    g.StudentCount,
		PresentDays:     g.PresentDays,
		SickDays:        g.SickDays,
		ExcusedDays:     g.ExcusedDays,
		UnexplainedDays: g.UnexplainedDays,
		AttendanceRate:  g.AttendanceRate,
	}
}
