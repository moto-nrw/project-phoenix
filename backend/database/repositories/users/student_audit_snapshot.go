package users

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// NativeStudentAuditSnapshot retains the exact tracked field projection used
// by the legacy root adapter, including ISO care-end dates and nil maps.
func NativeStudentAuditSnapshot(student *models.Student) *directory.StudentAuditSnapshot {
	if student == nil {
		return nil
	}
	snapshot := &directory.StudentAuditSnapshot{StudentID: student.ID, Status: string(student.Status), ExtraInfo: student.ExtraInfo, HealthInfo: student.HealthInfo, PickupStatus: student.PickupStatus, DepartureCompanionNote: student.DepartureCompanionNote, AllowedDepartureModes: student.AllowedDepartureModes, DepartureDays: student.DepartureDays}
	if student.EnrolledUntil != nil {
		snapshot.CareEnd = student.EnrolledUntil.String()
	}
	return snapshot
}
