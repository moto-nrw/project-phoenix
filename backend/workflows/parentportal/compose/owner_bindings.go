package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// PeopleDirectory is the People Directory surface the portal writes and reads
// guardian and student rows through.
type PeopleDirectory interface {
	peopledirectory.GuardianPortalCommand
	peopledirectory.StudentPortalCommand
	ListGuardiansByAccount(context.Context, []int64) ([]peopledirectory.Guardian, error)
}

func newGuardianRecords(people PeopleDirectory) care.GuardianRecords {
	return care.NewGuardianRecords(people, care.GuardianRecordRows[peopledirectory.GuardianContactRecord, peopledirectory.GuardianPhoneRecord, peopledirectory.GuardianContactLink, peopledirectory.GuardianLinkPickupPatch, peopledirectory.Guardian]{
		Contact: func(record care.GuardianContactRecord) peopledirectory.GuardianContactRecord {
			return peopledirectory.GuardianContactRecord(record)
		},
		Phone: func(record care.GuardianPhoneRecord) peopledirectory.GuardianPhoneRecord {
			return peopledirectory.GuardianPhoneRecord(record)
		},
		Link: func(record care.GuardianContactLink) peopledirectory.GuardianContactLink {
			return peopledirectory.GuardianContactLink(record)
		},
		Pickup: func(record care.GuardianLinkPickupPatch) peopledirectory.GuardianLinkPickupPatch {
			return peopledirectory.GuardianLinkPickupPatch(record)
		},
		Tenant: func(record peopledirectory.Guardian) int64 { return record.TenantID },
	}, peopledirectory.ErrGuardianEmailTaken)
}
func newStudentRecords(profiles careplan.StudentProfileCommands, people PeopleDirectory) care.StudentRecords {
	return care.NewStudentRecords(profiles, people, care.StudentRecordRows[careplan.StudentLiveStatus, peopledirectory.StudentPhotoState]{
		Live: func(studentID int64, sick bool, sickSince *time.Time, excused bool, excusedSince *time.Time) careplan.StudentLiveStatus {
			return careplan.StudentLiveStatus{StudentID: studentID, Sick: sick, SickSince: sickSince, Excused: excused, ExcusedSince: excusedSince}
		},
		Photo: func(record care.StudentPhotoState) peopledirectory.StudentPhotoState {
			return peopledirectory.StudentPhotoState(record)
		},
	})
}
func newGuardianChanges(log auditlog.GuardianChangeLog) care.GuardianChangeLog {
	return care.NewGuardianChanges(log, func(record care.GuardianChange) auditlog.GuardianChange { return auditlog.GuardianChange(record) })
}

// NewRequestSharePolicy binds the sharing ledger owner's pure visibility rule.
func NewRequestSharePolicy() peopledirectory.RequestSharePolicy {
	return peopledirectory.RequestSharePolicy{}
}
