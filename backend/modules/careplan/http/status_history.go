package careplanhttp

import (
	slog "log/slog"

	absencerecords "github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func boolPtrValue(v *bool) bool {
	return v != nil && *v
}

func newlyReportedAbsenceStatus(student *peopleModule.StudentState, wasSick, wasExcused bool) string {
	if !wasSick && boolPtrValue(student.Sick) {
		return absencerecords.StudentStatusDaySick
	}
	if !wasExcused && boolPtrValue(student.Excused) {
		return absencerecords.StudentStatusDayExcused
	}
	return ""
}

func (rs *Resource) logStatusHistoryError(studentID int64, err error) {
	if rs.Logger == nil || err == nil {
		return
	}
	rs.Logger.Error("student status history write failed",
		slog.Int64("student_id", studentID),
		slog.String("error", err.Error()),
	)
}
