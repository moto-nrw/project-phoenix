package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// PeopleDirectory is the People Directory surface the portal writes and reads
// guardian and student rows through.
type PeopleDirectory interface {
	peopledirectory.GuardianPortalCommand
	peopledirectory.StudentPortalCommand
	ListGuardiansByAccount(context.Context, []int64) ([]peopledirectory.Guardian, error)
}

// guardianRecords binds People Directory's guardian commands to the flows'
// port and reports a duplicate e-mail as the portal's conflict.
type guardianRecords struct{ people PeopleDirectory }

func (g guardianRecords) CreateGuardianContact(ctx context.Context, record care.GuardianContactRecord) (int64, error) {
	id, err := g.people.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord(record))
	return id, guardianError(err)
}

func (g guardianRecords) UpdateGuardianContact(ctx context.Context, guardianID int64, record care.GuardianContactRecord) error {
	return guardianError(g.people.UpdateGuardianContact(ctx, guardianID, peopledirectory.GuardianContactRecord(record)))
}

func (g guardianRecords) ReplaceGuardianPhones(ctx context.Context, guardianID int64, phones []care.GuardianPhoneRecord) error {
	records := make([]peopledirectory.GuardianPhoneRecord, 0, len(phones))
	for _, phone := range phones {
		records = append(records, peopledirectory.GuardianPhoneRecord(phone))
	}
	return g.people.ReplaceGuardianPhones(ctx, guardianID, records)
}

func (g guardianRecords) AddGuardianPhoneRecord(ctx context.Context, guardianID int64, phone care.GuardianPhoneRecord) (int64, error) {
	return g.people.AddGuardianPhoneRecord(ctx, guardianID, peopledirectory.GuardianPhoneRecord(phone))
}

func (g guardianRecords) SetGuardianPhoneNumber(ctx context.Context, phoneID int64, number string) error {
	return g.people.SetGuardianPhoneNumber(ctx, phoneID, number)
}

func (g guardianRecords) DeleteGuardianPhoneRecord(ctx context.Context, phoneID int64) error {
	return g.people.DeleteGuardianPhoneRecord(ctx, phoneID)
}

func (g guardianRecords) LinkGuardianContact(ctx context.Context, link care.GuardianContactLink) (int64, bool, error) {
	return g.people.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink(link))
}

func (g guardianRecords) PatchGuardianLinkPickup(ctx context.Context, patch care.GuardianLinkPickupPatch) (int64, error) {
	return g.people.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch(patch))
}

func (g guardianRecords) SetGuardianPortalLocale(ctx context.Context, accountID int64, locale string) (int64, error) {
	return g.people.SetGuardianPortalLocale(ctx, accountID, locale)
}

func (g guardianRecords) ListGuardianProfileTenants(ctx context.Context, accountID int64) ([]int64, error) {
	guardians, err := g.people.ListGuardiansByAccount(ctx, []int64{accountID})
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(guardians))
	tenantIDs := make([]int64, 0, len(guardians))
	for _, guardian := range guardians {
		if _, ok := seen[guardian.TenantID]; ok {
			continue
		}
		seen[guardian.TenantID] = struct{}{}
		tenantIDs = append(tenantIDs, guardian.TenantID)
	}
	return tenantIDs, nil
}

func guardianError(err error) error {
	if errors.Is(err, peopledirectory.ErrGuardianEmailTaken) {
		return errors.Join(care.ErrGuardianEmailConflict, err)
	}
	return err
}

// studentRecords binds the writes of the child's own rows: Care Plan owns the
// care profile, People Directory the photo consent.
type studentRecords struct {
	care   careplan.StudentProfileCommands
	people PeopleDirectory
}

func (s studentRecords) SetStudentHealthInfo(ctx context.Context, studentID int64, healthInfo *string) error {
	rows, err := s.care.SetStudentHealthInfo(ctx, studentID, healthInfo)
	return requireCareProfileWrite(studentID, rows, err)
}

// SetStudentLiveAbsence writes the four live flags as given; an unset flag is
// stored as false, as the full care-row write stored it.
func (s studentRecords) SetStudentLiveAbsence(ctx context.Context, absence care.StudentLiveAbsence) error {
	rows, err := s.care.SetStudentLiveStatus(ctx, careplan.StudentLiveStatus{
		StudentID: absence.StudentID, Sick: absence.Sick != nil && *absence.Sick, SickSince: absence.SickSince,
		Excused: absence.Excused != nil && *absence.Excused, ExcusedSince: absence.ExcusedSince,
	})
	return requireCareProfileWrite(absence.StudentID, rows, err)
}

// requireCareProfileWrite fails loudly when Care Plan changed no care row: a
// child the flow just locked always has one, so zero rows is a data fault that
// must not pass as a saved change.
func requireCareProfileWrite(studentID, rows int64, err error) error {
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("parent portal: child %d has no care profile to write", studentID)
	}
	return nil
}

func (s studentRecords) SetStudentPhotoConsent(ctx context.Context, photo care.StudentPhotoState) error {
	return s.people.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState(photo))
}

// guardianChanges appends the guardian change trail through the Audit
// Platform, inside the flow's tenant unit of work.
type guardianChanges struct{ log auditlog.GuardianChangeLog }

func (g guardianChanges) RecordGuardianChanges(ctx context.Context, changes []care.GuardianChange) error {
	if len(changes) == 0 {
		return nil
	}
	rows := make([]auditlog.GuardianChange, 0, len(changes))
	for _, change := range changes {
		rows = append(rows, auditlog.GuardianChange(change))
	}
	return g.log.RecordGuardianChanges(ctx, rows)
}
