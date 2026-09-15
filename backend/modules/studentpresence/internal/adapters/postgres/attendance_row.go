package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

type attendanceRow struct {
	bun.BaseModel      `bun:"table:active.attendance,alias:attendance"`
	ID                 int64      `bun:"id,pk,autoincrement"`
	TenantID           int64      `bun:"tenant_id,notnull"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID          int64      `bun:"student_id,notnull"`
	Date               ports.Date `bun:"date,notnull,type:date"`
	CheckInTime        time.Time  `bun:"check_in_time,notnull"`
	CheckOutTime       *time.Time `bun:"check_out_time"`
	CheckedInBy        int64      `bun:"checked_in_by,nullzero"`
	CheckedOutBy       *int64     `bun:"checked_out_by"`
	DeviceID           int64      `bun:"device_id,notnull"`
	CheckedOutDeviceID *int64     `bun:"checked_out_device_id"`
	YardSince          *time.Time `bun:"yard_since"`
}

func attendanceRowFromRecord(row *ports.Attendance) *attendanceRow {
	return &attendanceRow{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, Date: row.Date, CheckInTime: row.CheckInTime, CheckOutTime: row.CheckOutTime,
		CheckedInBy: row.CheckedInBy, CheckedOutBy: row.CheckedOutBy, DeviceID: row.DeviceID,
		CheckedOutDeviceID: row.CheckedOutDeviceID, YardSince: row.YardSince}
}

func (row *attendanceRow) record() *ports.Attendance {
	return &ports.Attendance{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, Date: row.Date, CheckInTime: row.CheckInTime, CheckOutTime: row.CheckOutTime,
		CheckedInBy: row.CheckedInBy, CheckedOutBy: row.CheckedOutBy, DeviceID: row.DeviceID,
		CheckedOutDeviceID: row.CheckedOutDeviceID, YardSince: row.YardSince}
}

func attendanceRecordsFromRows(rows []*attendanceRow) []*ports.Attendance {
	result := make([]*ports.Attendance, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.record())
	}
	return result
}
