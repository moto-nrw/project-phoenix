package care

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func portalRecord[A, B any](value A, convert func(A) B) B { return convert(value) }

type GuardianSource[C, P, L, K, G any] interface {
	CreateGuardianContact(context.Context, C) (int64, error)
	UpdateGuardianContact(context.Context, int64, C) error
	ReplaceGuardianPhones(context.Context, int64, []P) error
	AddGuardianPhoneRecord(context.Context, int64, P) (int64, error)
	SetGuardianPhoneNumber(context.Context, int64, string) error
	DeleteGuardianPhoneRecord(context.Context, int64) error
	LinkGuardianContact(context.Context, L) (int64, bool, error)
	PatchGuardianLinkPickup(context.Context, K) (int64, error)
	SetGuardianPortalLocale(context.Context, int64, string) (int64, error)
	ListGuardiansByAccount(context.Context, []int64) ([]G, error)
}
type GuardianRecordRows[C, P, L, K, G any] struct {
	Contact func(GuardianContactRecord) C
	Phone   func(GuardianPhoneRecord) P
	Link    func(GuardianContactLink) L
	Pickup  func(GuardianLinkPickupPatch) K
	Tenant  func(G) int64
}
type guardianRecords[C, P, L, K, G any] struct {
	GuardianSource[C, P, L, K, G]
	rows           GuardianRecordRows[C, P, L, K, G]
	duplicateEmail error
}

func NewGuardianRecords[C, P, L, K, G any](people GuardianSource[C, P, L, K, G], rows GuardianRecordRows[C, P, L, K, G], duplicateEmail error) GuardianRecords {
	return guardianRecords[C, P, L, K, G]{GuardianSource: people, rows: rows, duplicateEmail: duplicateEmail}
}

type StudentCare[C any] interface {
	SetStudentHealthInfo(context.Context, int64, *string) (int64, error)
	SetStudentLiveStatus(context.Context, C) (int64, error)
}
type StudentPeople[P any] interface {
	SetStudentPhotoConsent(context.Context, P) error
}
type StudentRecordRows[C, P any] struct {
	Live  func(int64, bool, *time.Time, bool, *time.Time) C
	Photo func(StudentPhotoState) P
}
type studentRecords[C, P any] struct {
	care   StudentCare[C]
	people StudentPeople[P]
	rows   StudentRecordRows[C, P]
}

func NewStudentRecords[C, P any](care StudentCare[C], people StudentPeople[P], rows StudentRecordRows[C, P]) StudentRecords {
	return studentRecords[C, P]{care: care, people: people, rows: rows}
}
func portalLiveAbsence[C any](absence StudentLiveAbsence, convert func(int64, bool, *time.Time, bool, *time.Time) C) C {
	return convert(absence.StudentID, absence.Sick != nil && *absence.Sick, absence.SickSince, absence.Excused != nil && *absence.Excused, absence.ExcusedSince)
}

type GuardianChangeSource[C any] interface {
	RecordGuardianChanges(context.Context, []C) error
}
type guardianChanges[C any] struct {
	log     GuardianChangeSource[C]
	convert func(GuardianChange) C
}

func NewGuardianChanges[C any](log GuardianChangeSource[C], convert func(GuardianChange) C) GuardianChangeLog {
	return guardianChanges[C]{log: log, convert: convert}
}

func (g guardianRecords[C, P, L, K, G]) CreateGuardianContact(ctx context.Context, record GuardianContactRecord) (int64, error) {
	id, err := g.GuardianSource.CreateGuardianContact(ctx, portalRecord(record, g.rows.Contact))
	return id, guardianError(err, g.duplicateEmail)
}

func (g guardianRecords[C, P, L, K, G]) UpdateGuardianContact(ctx context.Context, guardianID int64, record GuardianContactRecord) error {
	return guardianError(g.GuardianSource.UpdateGuardianContact(ctx, guardianID, portalRecord(record, g.rows.Contact)), g.duplicateEmail)
}

func (g guardianRecords[C, P, L, K, G]) ReplaceGuardianPhones(ctx context.Context, guardianID int64, phones []GuardianPhoneRecord) error {
	records := make([]P, 0, len(phones))
	for _, phone := range phones {
		records = append(records, portalRecord(phone, g.rows.Phone))
	}
	return g.GuardianSource.ReplaceGuardianPhones(ctx, guardianID, records)
}

func (g guardianRecords[C, P, L, K, G]) AddGuardianPhoneRecord(ctx context.Context, guardianID int64, phone GuardianPhoneRecord) (int64, error) {
	return g.GuardianSource.AddGuardianPhoneRecord(ctx, guardianID, portalRecord(phone, g.rows.Phone))
}

func (g guardianRecords[C, P, L, K, G]) LinkGuardianContact(ctx context.Context, link GuardianContactLink) (int64, bool, error) {
	return g.GuardianSource.LinkGuardianContact(ctx, portalRecord(link, g.rows.Link))
}

func (g guardianRecords[C, P, L, K, G]) PatchGuardianLinkPickup(ctx context.Context, patch GuardianLinkPickupPatch) (int64, error) {
	return g.GuardianSource.PatchGuardianLinkPickup(ctx, portalRecord(patch, g.rows.Pickup))
}

func (g guardianRecords[C, P, L, K, G]) ListGuardianProfileTenants(ctx context.Context, accountID int64) ([]int64, error) {
	guardians, err := g.ListGuardiansByAccount(ctx, []int64{accountID})
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(guardians))
	tenantIDs := make([]int64, 0, len(guardians))
	for _, guardian := range guardians {
		if _, ok := seen[portalRecord(guardian, g.rows.Tenant)]; ok {
			continue
		}
		seen[portalRecord(guardian, g.rows.Tenant)] = struct{}{}
		tenantIDs = append(tenantIDs, portalRecord(guardian, g.rows.Tenant))
	}
	return tenantIDs, nil
}

func guardianError(err error, duplicateEmail error) error {
	if errors.Is(err, duplicateEmail) {
		return errors.Join(ErrGuardianEmailConflict, err)
	}
	return err
}

func (s studentRecords[C, P]) SetStudentHealthInfo(ctx context.Context, studentID int64, healthInfo *string) error {
	rows, err := s.care.SetStudentHealthInfo(ctx, studentID, healthInfo)
	return requireCareProfileWrite(studentID, rows, err)
}

// SetStudentLiveAbsence writes the four live flags as given; an unset flag is
// stored as false, as the full care-row write stored it.
func (s studentRecords[C, P]) SetStudentLiveAbsence(ctx context.Context, absence StudentLiveAbsence) error {
	rows, err := s.care.SetStudentLiveStatus(ctx, portalLiveAbsence(absence, s.rows.Live))
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

func (s studentRecords[C, P]) SetStudentPhotoConsent(ctx context.Context, photo StudentPhotoState) error {
	return s.people.SetStudentPhotoConsent(ctx, portalRecord(photo, s.rows.Photo))
}

func (g guardianChanges[C]) RecordGuardianChanges(ctx context.Context, changes []GuardianChange) error {
	rows := make([]C, 0, len(changes))
	for _, change := range changes {
		rows = append(rows, portalRecord(change, g.convert))
	}
	return g.log.RecordGuardianChanges(ctx, rows)
}
