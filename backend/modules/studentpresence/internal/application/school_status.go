package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type SchoolStatus struct {
	StudentID                            int64
	CheckedInBy                          int64
	CheckedOutBy                         *int64
	Date, Status                         string
	CheckInTime, CheckOutTime, YardSince *time.Time
}

func (s *Service) ListSchoolStatuses(ctx context.Context, ids []int64, date string) (result []SchoolStatus, err error) {
	err = s.run("list_school_statuses", func() (ports.Stats, error) {
		if date == "" || !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid school status query")
		}
		result = make([]SchoolStatus, 0, len(ids))
		if len(ids) == 0 {
			return ports.Stats{}, nil
		}
		rows, stats, err := s.store.ListAttendance(ctx, ports.AttendanceFilter{
			StudentIDs: ids, FromDate: date, UntilDate: date, StudentOrder: true, NewestFirst: true,
		})
		if err != nil {
			return stats, err
		}
		latest := make(map[int64]*ports.Attendance, len(rows))
		for _, row := range rows {
			if _, found := latest[row.StudentID]; !found {
				latest[row.StudentID] = row
			}
		}
		for _, id := range ids {
			status := SchoolStatus{StudentID: id, Date: date, Status: "not_checked_in"}
			if row := latest[id]; row != nil {
				status.CheckedInBy, status.CheckedOutBy = row.CheckedInBy, row.CheckedOutBy
				status.CheckInTime, status.CheckOutTime, status.YardSince = &row.CheckInTime, row.CheckOutTime, row.YardSince
				switch {
				case row.CheckOutTime != nil:
					status.Status = "checked_out"
				case row.YardSince != nil:
					status.Status = "on_yard"
				default:
					status.Status = "checked_in"
				}
			}
			result = append(result, status)
		}
		return stats, nil
	})
	return result, err
}
