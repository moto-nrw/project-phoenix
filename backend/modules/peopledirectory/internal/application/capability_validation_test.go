package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// recordingEngine records the persistence calls behind the owner validation tests.
// Unused port methods are intentionally unbound: an unexpected call fails the test.
type recordingEngine struct {
	ports.Store
	created       peopledirectory.CreatePerson
	listed        []int64
	across        bool
	searched      peopledirectory.PersonFilter
	tag           string
	calls         int
	student       studentCall
	guardian      guardianCall
	observed      []string
	directory     peopledirectory.StudentDirectoryFilter
	directoryRows []peopledirectory.StudentRecord
	lockedRecord  int64
	recordIDs     []int64
	details       peopledirectory.PersonDetails
}

type validationTransaction struct {
	ports.Transaction
	recording *recordingEngine
}

func (t validationTransaction) RunRead(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (t validationTransaction) RunWrite(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (t validationTransaction) RunAdminRead(ctx context.Context, fn func(context.Context) error) error {
	t.recording.recordAcross()
	return fn(ctx)
}
func (t validationTransaction) RequireTransaction(context.Context) error { return nil }
func (t validationTransaction) RunSavepoint(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func newValidationService(recording *recordingEngine, providers ...peopledirectory.GuardianProvider) *Service {
	tx := validationTransaction{recording: recording}
	observe := func(o ports.Observation) {
		code := peopledirectory.ErrorCode(o.Err)
		if errors.Is(o.Err, peopledirectory.ErrGuardianProviderUnbound) {
			code = "unbound"
		}
		recording.observed = append(recording.observed, o.Operation+":"+code)
	}
	return New(recording, tx, observe, Services{
		Students:  &StudentService{store: validationStudentStore{recording: recording}, tx: tx, observe: observe},
		Guardians: &GuardianService{store: validationGuardianStore{recording: recording}, tx: tx, observe: observe, owners: validationLinkOwners{}},
	}, providers...)
}

func (e *recordingEngine) Create(_ context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, domain.OperationStats, error) {
	e.calls++
	e.created = input
	return peopledirectory.Person{FirstName: input.FirstName, LastName: input.LastName, TagID: input.TagID}, domain.OperationStats{}, nil
}
func (e *recordingEngine) FindByID(context.Context, int64, string) (peopledirectory.Person, bool, domain.OperationStats, error) {
	return peopledirectory.Person{ID: 7}, true, domain.OperationStats{}, nil
}
func (e *recordingEngine) FindByTag(_ context.Context, tag string) (peopledirectory.Person, bool, domain.OperationStats, error) {
	e.calls++
	e.tag = tag
	return peopledirectory.Person{ID: 7}, true, domain.OperationStats{}, nil
}
func (e *recordingEngine) ListByIDs(_ context.Context, ids []int64) ([]peopledirectory.Person, domain.OperationStats, error) {
	e.calls++
	e.listed = ids
	return nil, domain.OperationStats{}, nil
}
func (e *recordingEngine) ListByTenantIDs(_ context.Context, ids []int64) ([]peopledirectory.Person, domain.OperationStats, error) {
	e.calls++
	e.listed = ids
	return nil, domain.OperationStats{}, nil
}
func (e *recordingEngine) Search(_ context.Context, f peopledirectory.PersonFilter) ([]peopledirectory.Person, domain.OperationStats, error) {
	e.calls++
	e.searched = f
	return nil, domain.OperationStats{}, nil
}
func (e *recordingEngine) FindByAccount(context.Context, int64) (peopledirectory.Person, bool, domain.OperationStats, error) {
	return peopledirectory.Person{}, false, domain.OperationStats{}, nil
}
func (e *recordingEngine) SetTag(_ context.Context, _ int64, tag *string) (domain.OperationStats, error) {
	e.calls++
	e.tag = *tag
	return domain.OperationStats{}, nil
}
func (e *recordingEngine) SetNames(context.Context, int64, string, string) (domain.OperationStats, error) {
	e.calls++
	return domain.OperationStats{}, nil
}
func (e *recordingEngine) SetDetails(_ context.Context, _ int64, first, last, birthday string) (domain.OperationStats, error) {
	e.calls++
	e.details = peopledirectory.PersonDetails{FirstName: first, LastName: last, Birthday: birthday}
	return domain.OperationStats{}, nil
}

type validationStudentStore struct {
	ports.StudentStore
	recording *recordingEngine
}

func (s validationStudentStore) ListByIDs(_ context.Context, ids []int64) ([]peopledirectory.Student, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordStudent(studentCall{ids: ids})
	return nil, domain.OperationStats{}, nil
}
func (s validationStudentStore) ListNamesByIDs(_ context.Context, ids []int64) ([]peopledirectory.StudentName, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordStudent(studentCall{ids: ids})
	return nil, domain.OperationStats{}, nil
}
func (s validationStudentStore) ListByClasses(_ context.Context, classes []string) ([]peopledirectory.Student, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordStudent(studentCall{classes: classes})
	return nil, domain.OperationStats{}, nil
}
func (s validationStudentStore) Lock(_ context.Context, id int64) (bool, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordStudent(studentCall{ids: []int64{id}})
	return true, domain.OperationStats{}, nil
}
func (s validationStudentStore) ListDirectory(_ context.Context, f domain.StudentDirectoryFilter) ([]domain.StudentRecord, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordDirectory(publicDirectoryFilter(f))
	rows := make([]domain.StudentRecord, 0, len(s.recording.directoryRows))
	for _, row := range s.recording.directoryRows {
		rows = append(rows, domain.StudentRecord(row))
	}
	return rows, domain.OperationStats{}, nil
}
func (s validationStudentStore) CountDirectory(_ context.Context, f domain.StudentDirectoryFilter) (int, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordDirectory(publicDirectoryFilter(f))
	return 0, domain.OperationStats{}, nil
}
func (s validationStudentStore) ListRecordsByIDs(_ context.Context, ids []int64) ([]domain.StudentRecord, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordRecordids(ids)
	return nil, domain.OperationStats{}, nil
}
func (s validationStudentStore) FindRecord(_ context.Context, id int64, _ string) (domain.StudentRecord, bool, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordLockedrecord(id)
	return domain.StudentRecord{}, true, domain.OperationStats{}, nil
}
func (s validationStudentStore) LockRecord(ctx context.Context, id int64, lock string) (domain.StudentRecord, bool, domain.OperationStats, error) {
	return s.FindRecord(ctx, id, lock)
}
func (s validationStudentStore) LockClassWrites(context.Context) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}
func publicDirectoryFilter(f domain.StudentDirectoryFilter) peopledirectory.StudentDirectoryFilter {
	return peopledirectory.StudentDirectoryFilter{IDs: f.IDs, KeepAlumni: f.KeepAlumni, SchoolClasses: f.SchoolClasses, Page: f.Page, PageSize: f.PageSize, CareStatus: f.CareStatus, CareStatusOn: f.CareStatusOn}
}

type validationGuardianStore struct {
	ports.GuardianStore
	recording *recordingEngine
}

func (s validationGuardianStore) ListLinksByAccounts(_ context.Context, ids []int64) ([]peopledirectory.GuardianLink, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordGuardianAccountids(ids)
	return nil, domain.OperationStats{}, nil
}
func (s validationGuardianStore) ListByIDs(_ context.Context, ids []int64) ([]domain.Guardian, domain.OperationStats, error) {
	s.recording.calls++
	s.recording.recordGuardianIds(ids)
	return nil, domain.OperationStats{}, nil
}
func (s validationGuardianStore) UpdateLinkPickup(context.Context, domain.GuardianLinkPickupPatch) (bool, domain.OperationStats, error) {
	s.recording.calls++
	return true, domain.OperationStats{}, nil
}
func (s validationGuardianStore) SetPortalLocale(context.Context, int64, string) (int64, domain.OperationStats, error) {
	s.recording.calls++
	return 0, domain.OperationStats{}, nil
}

func (t validationTransaction) TenantID(context.Context) int64 { return 1 }
func (s validationStudentStore) LockEnrollmentClassWrites(context.Context) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

type validationLinkOwners struct{ ports.GuardianLinkOwners }

func (validationLinkOwners) ChangeGuardianPickupPermission(context.Context, int64, int64, *bool, bool, *string) (bool, error) {
	return true, nil
}
func (s validationGuardianStore) LockLinkForPickup(context.Context, int64, *bool) (bool, domain.OperationStats, error) {
	s.recording.calls++
	return true, domain.OperationStats{}, nil
}

func (e *recordingEngine) recordStudent(value studentCall) { e.student = value }

func (e *recordingEngine) recordDirectory(value peopledirectory.StudentDirectoryFilter) {
	e.directory = value
}

func (e *recordingEngine) recordRecordids(value []int64) { e.recordIDs = value }

func (e *recordingEngine) recordLockedrecord(value int64) { e.lockedRecord = value }

func (e *recordingEngine) recordGuardianAccountids(value []int64) { e.guardian.accountIDs = value }

func (e *recordingEngine) recordGuardianIds(value []int64) { e.guardian.ids = value }

func (e *recordingEngine) recordAcross() { e.across = true }
