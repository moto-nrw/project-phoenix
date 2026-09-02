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

type courseResponse struct {
	CourseID           string   `json:"course_id"`
	Name               string   `json:"name"`
	CategoryName       string   `json:"category_name"`
	MaxParticipants    int      `json:"max_participants"`
	HeldInstances      int      `json:"held_instances"`
	CancelledInstances int      `json:"cancelled_instances"`
	StudentCount       int      `json:"student_count"`
	PresentDays        int      `json:"present_days"`
	AbsentDays         int      `json:"absent_days"`
	OpenDays           int      `json:"open_days"`
	ParticipationRate  *float64 `json:"participation_rate"`
	OccupancyPercent   *float64 `json:"occupancy_percent"`
}

type courseStudentResponse struct {
	StudentID         string   `json:"student_id"`
	FirstName         string   `json:"first_name"`
	LastName          string   `json:"last_name"`
	SchoolClass       string   `json:"school_class"`
	GroupName         string   `json:"group_name"`
	CourseID          string   `json:"course_id"`
	CourseName        string   `json:"course_name"`
	PresentDays       int      `json:"present_days"`
	AbsentDays        int      `json:"absent_days"`
	OpenDays          int      `json:"open_days"`
	ParticipationRate *float64 `json:"participation_rate"`
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

	Courses        []courseResponse        `json:"courses"`
	CourseStudents []courseStudentResponse `json:"course_students"`
	CourseTotals   courseResponse          `json:"course_totals"`
	CourseDataDays int                     `json:"course_data_days"`
	CourseDataFrom string                  `json:"course_data_from"`
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

		Courses:        make([]courseResponse, 0, len(report.Courses)),
		CourseStudents: make([]courseStudentResponse, 0, len(report.CourseStudents)),
		CourseTotals:   toCourseResponse(report.CourseTotals),
		CourseDataDays: report.CourseDataDays,
		CourseDataFrom: report.CourseDataFrom.String(),
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
	for _, course := range report.Courses {
		out.Courses = append(out.Courses, toCourseResponse(course))
	}
	for _, row := range report.CourseStudents {
		out.CourseStudents = append(out.CourseStudents, courseStudentResponse{
			StudentID:         strconv.FormatInt(row.StudentID, 10),
			FirstName:         row.FirstName,
			LastName:          row.LastName,
			SchoolClass:       row.SchoolClass,
			GroupName:         row.GroupName,
			CourseID:          strconv.FormatInt(row.CourseID, 10),
			CourseName:        row.CourseName,
			PresentDays:       row.PresentDays,
			AbsentDays:        row.AbsentDays,
			OpenDays:          row.OpenDays,
			ParticipationRate: row.ParticipationRate,
		})
	}
	return out
}

func toCourseResponse(course statisticsService.CourseRow) courseResponse {
	return courseResponse{
		CourseID:           strconv.FormatInt(course.CourseID, 10),
		Name:               course.Name,
		CategoryName:       course.CategoryName,
		MaxParticipants:    course.MaxParticipants,
		HeldInstances:      course.HeldInstances,
		CancelledInstances: course.CancelledInstances,
		StudentCount:       course.StudentCount,
		PresentDays:        course.PresentDays,
		AbsentDays:         course.AbsentDays,
		OpenDays:           course.OpenDays,
		ParticipationRate:  course.ParticipationRate,
		OccupancyPercent:   course.OccupancyPercent,
	}
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
