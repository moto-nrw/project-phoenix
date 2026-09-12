package ports

import (
	"context"
	"time"
)

// Read methods are forwarded unchanged. Only owner commands emit observations.
type observedPersonDirectory struct{ PersonDirectory }

func ObservePersonDirectory(source PersonDirectory) PersonDirectory {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedPersonDirectory); observed {
		return source
	}
	return observedPersonDirectory{source}
}

func (o observedPersonDirectory) CreatePerson(ctx context.Context, input CreatePerson) (result Person, err error) {
	defer observeCommand(ctx, "people-directory", "create_person", time.Now(), &err)
	return o.PersonDirectory.CreatePerson(ctx, input)
}

func (o observedPersonDirectory) UpdatePerson(ctx context.Context, input UpdatePerson) (result Person, err error) {
	defer observeCommand(ctx, "people-directory", "update_person", time.Now(), &err)
	return o.PersonDirectory.UpdatePerson(ctx, input)
}

type observedStudentDirectory struct{ StudentDirectory }

func ObserveStudentDirectory(source StudentDirectory) StudentDirectory {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedStudentDirectory); observed {
		return source
	}
	return observedStudentDirectory{source}
}

func (o observedStudentDirectory) CreateEnrollmentStudent(ctx context.Context, input EnrollmentStudent) (result CreatedEnrollmentStudent, err error) {
	defer observeCommand(ctx, "people-directory", "create_enrollment_student", time.Now(), &err)
	return o.StudentDirectory.CreateEnrollmentStudent(ctx, input)
}

func (o observedStudentDirectory) RenewEnrollmentStudent(ctx context.Context, id int64, input EnrollmentStudent) (err error) {
	defer observeCommand(ctx, "people-directory", "renew_enrollment_student", time.Now(), &err)
	return o.StudentDirectory.RenewEnrollmentStudent(ctx, id, input)
}

func (o observedStudentDirectory) ApplyEnrollmentProfile(ctx context.Context, id int64, input EnrollmentProfilePatch) (err error) {
	defer observeCommand(ctx, "people-directory", "apply_enrollment_profile", time.Now(), &err)
	return o.StudentDirectory.ApplyEnrollmentProfile(ctx, id, input)
}

type observedGuardianDirectory struct{ GuardianDirectory }

func ObserveGuardianDirectory(source GuardianDirectory) GuardianDirectory {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedGuardianDirectory); observed {
		return source
	}
	return observedGuardianDirectory{source}
}

func (o observedGuardianDirectory) CreateGuardian(ctx context.Context, input GuardianInput) (result Guardian, err error) {
	defer observeCommand(ctx, "people-directory", "create_guardian", time.Now(), &err)
	return o.GuardianDirectory.CreateGuardian(ctx, input)
}

func (o observedGuardianDirectory) UpdateGuardian(ctx context.Context, id int64, input GuardianInput) (err error) {
	defer observeCommand(ctx, "people-directory", "update_guardian", time.Now(), &err)
	return o.GuardianDirectory.UpdateGuardian(ctx, id, input)
}

func (o observedGuardianDirectory) AddGuardianPhone(ctx context.Context, id int64, input GuardianPhoneInput) (result GuardianPhone, err error) {
	defer observeCommand(ctx, "people-directory", "add_guardian_phone", time.Now(), &err)
	return o.GuardianDirectory.AddGuardianPhone(ctx, id, input)
}

func (o observedGuardianDirectory) LinkGuardianToStudent(ctx context.Context, input LinkGuardian) (result GuardianLink, err error) {
	defer observeCommand(ctx, "people-directory", "link_guardian_to_student", time.Now(), &err)
	return o.GuardianDirectory.LinkGuardianToStudent(ctx, input)
}

func (o observedGuardianDirectory) UpdateGuardianLink(ctx context.Context, id int64, input GuardianLinkUpdate) (err error) {
	defer observeCommand(ctx, "people-directory", "update_guardian_link", time.Now(), &err)
	return o.GuardianDirectory.UpdateGuardianLink(ctx, id, input)
}

type observedStudentSchedules struct{ StudentSchedules }

func ObserveStudentSchedules(source StudentSchedules) StudentSchedules {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedStudentSchedules); observed {
		return source
	}
	return observedStudentSchedules{source}
}

func (o observedStudentSchedules) CreateArrivalSchedule(ctx context.Context, input ArrivalSchedule) (result ArrivalSchedule, err error) {
	defer observeCommand(ctx, "care-plan", "create_arrival_schedule", time.Now(), &err)
	return o.StudentSchedules.CreateArrivalSchedule(ctx, input)
}

func (o observedStudentSchedules) UpdateArrivalSchedule(ctx context.Context, input ArrivalSchedule) (err error) {
	defer observeCommand(ctx, "care-plan", "update_arrival_schedule", time.Now(), &err)
	return o.StudentSchedules.UpdateArrivalSchedule(ctx, input)
}

func (o observedStudentSchedules) CreatePickupSchedule(ctx context.Context, input PickupSchedule) (result PickupSchedule, err error) {
	defer observeCommand(ctx, "care-plan", "create_pickup_schedule", time.Now(), &err)
	return o.StudentSchedules.CreatePickupSchedule(ctx, input)
}

func (o observedStudentSchedules) UpdatePickupSchedule(ctx context.Context, input PickupSchedule) (err error) {
	defer observeCommand(ctx, "care-plan", "update_pickup_schedule", time.Now(), &err)
	return o.StudentSchedules.UpdatePickupSchedule(ctx, input)
}

type observedPrivacyConsents struct{ PrivacyConsents }

func ObservePrivacyConsents(source PrivacyConsents) PrivacyConsents {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedPrivacyConsents); observed {
		return source
	}
	return observedPrivacyConsents{source}
}

func (o observedPrivacyConsents) RecordPrivacyConsent(ctx context.Context, input PrivacyConsent) (result PrivacyConsent, err error) {
	defer observeCommand(ctx, "student-presence", "record_privacy_consent", time.Now(), &err)
	return o.PrivacyConsents.RecordPrivacyConsent(ctx, input)
}

type observedStaffMembership struct{ StaffMembership }

func ObserveStaffMembership(source StaffMembership) StaffMembership {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedStaffMembership); observed {
		return source
	}
	return observedStaffMembership{source}
}

func (o observedStaffMembership) CreateStaff(ctx context.Context, input CreateStaff) (result Staff, err error) {
	defer observeCommand(ctx, "school-membership", "create_staff", time.Now(), &err)
	return o.StaffMembership.CreateStaff(ctx, input)
}

func (o observedStaffMembership) UpdateStaff(ctx context.Context, input UpdateStaff) (result Staff, err error) {
	defer observeCommand(ctx, "school-membership", "update_staff", time.Now(), &err)
	return o.StaffMembership.UpdateStaff(ctx, input)
}

func (o observedStaffMembership) CreateTeacher(ctx context.Context, input CreateTeacher) (result Teacher, err error) {
	defer observeCommand(ctx, "school-membership", "create_teacher", time.Now(), &err)
	return o.StaffMembership.CreateTeacher(ctx, input)
}

func (o observedStaffMembership) UpdateTeacher(ctx context.Context, input UpdateTeacher) (result Teacher, err error) {
	defer observeCommand(ctx, "school-membership", "update_teacher", time.Now(), &err)
	return o.StaffMembership.UpdateTeacher(ctx, input)
}

type observedClassListMembership struct{ ClassListMembership }

func ObserveClassListMembership(source ClassListMembership) ClassListMembership {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedClassListMembership); observed {
		return source
	}
	return observedClassListMembership{source}
}

func (o observedClassListMembership) CreateClassListEntry(ctx context.Context, input CreateClassListEntry) (result ClassListEntry, err error) {
	defer observeCommand(ctx, "school-membership", "create_class_list_entry", time.Now(), &err)
	return o.ClassListMembership.CreateClassListEntry(ctx, input)
}

type observedStaffRecords struct{ StaffRecords }

func ObserveStaffRecords(source StaffRecords) StaffRecords {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedStaffRecords); observed {
		return source
	}
	return observedStaffRecords{source}
}

func (o observedStaffRecords) CreateStaffMasterData(ctx context.Context, input StaffMasterData) (result StaffMasterData, err error) {
	defer observeCommand(ctx, "workforce", "create_staff_master_data", time.Now(), &err)
	return o.StaffRecords.CreateStaffMasterData(ctx, input)
}

func (o observedStaffRecords) UpdateStaffMasterData(ctx context.Context, input StaffMasterData) (result StaffMasterData, err error) {
	defer observeCommand(ctx, "workforce", "update_staff_master_data", time.Now(), &err)
	return o.StaffRecords.UpdateStaffMasterData(ctx, input)
}

func (o observedStaffRecords) ReplaceStaffQualifications(ctx context.Context, id int64, qualifications []StaffQualification) (result []StaffQualification, err error) {
	defer observeCommand(ctx, "workforce", "replace_staff_qualifications", time.Now(), &err)
	return o.StaffRecords.ReplaceStaffQualifications(ctx, id, qualifications)
}

type observedOpeningBalanceBookings struct{ OpeningBalanceBookings }

func ObserveOpeningBalanceBookings(source OpeningBalanceBookings) OpeningBalanceBookings {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedOpeningBalanceBookings); observed {
		return source
	}
	return observedOpeningBalanceBookings{source}
}

func (o observedOpeningBalanceBookings) CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, date string, minutes int, note string) (result *StaffBalanceAdjustment, err error) {
	defer observeCommand(ctx, "workforce", "create_opening_balance", time.Now(), &err)
	return o.OpeningBalanceBookings.CreateOpeningBalance(ctx, staffID, decidedBy, date, minutes, note)
}

type observedVacationTakeovers struct{ VacationTakeovers }

func ObserveVacationTakeovers(source VacationTakeovers) VacationTakeovers {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedVacationTakeovers); observed {
		return source
	}
	return observedVacationTakeovers{source}
}

func (o observedVacationTakeovers) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) (err error) {
	defer observeCommand(ctx, "workforce", "upsert_vacation_quota", time.Now(), &err)
	return o.VacationTakeovers.UpsertVacationQuota(ctx, staffID, year, entitled, carryover)
}

func (o observedVacationTakeovers) SetVacationOpening(ctx context.Context, staffID, decidedBy int64, input SetVacationOpeningRequest) (result *StaffVacationOpening, err error) {
	defer observeCommand(ctx, "workforce", "set_vacation_opening", time.Now(), &err)
	return o.VacationTakeovers.SetVacationOpening(ctx, staffID, decidedBy, input)
}

type observedAuditCommand struct{ AuditCommand }

func ObserveAuditCommand(source AuditCommand) AuditCommand {
	if source == nil {
		return nil
	}
	if _, observed := source.(observedAuditCommand); observed {
		return source
	}
	return observedAuditCommand{source}
}

func (o observedAuditCommand) Append(ctx context.Context, event any) (err error) {
	defer observeCommand(ctx, "audit-platform", "append", time.Now(), &err)
	return o.AuditCommand.Append(ctx, event)
}
