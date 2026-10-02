package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the orchestration rules of the personnel-record
// administration against in-memory ports: no change without an audit row
// written first, no sensitive read without an access-log row, and no write
// after a failed audit. The database-backed behaviour is covered where the
// composition binds the real repositories.

var errAuditDown = errors.New("audit trail unavailable")

// eventLog records the order in which the ports were called.
type eventLog struct{ events []string }

func (l *eventLog) add(event string) { l.events = append(l.events, event) }

type fakeStore struct {
	ports.Store
	log        *eventLog
	masterData *domain.StaffMasterData
	financial  *domain.StaffFinancialData
	documents  []domain.StaffDocument
	queued     []domain.StaffDocumentFileCleanup
}

func (s *fakeStore) FindStaffMasterData(_ context.Context, staffID int64) (domain.StaffMasterData, bool, domain.OperationStats, error) {
	if s.masterData == nil || s.masterData.StaffID != staffID {
		return domain.StaffMasterData{}, false, domain.OperationStats{}, nil
	}
	return *s.masterData, true, domain.OperationStats{}, nil
}

func (s *fakeStore) CreateStaffMasterData(_ context.Context, value domain.StaffMasterData) (domain.StaffMasterData, domain.OperationStats, error) {
	s.log.add("create master data")
	value.ID = 1
	s.masterData = &value
	return value, domain.OperationStats{}, nil
}

func (s *fakeStore) UpdateStaffMasterData(_ context.Context, value domain.StaffMasterData) (domain.StaffMasterData, bool, domain.OperationStats, error) {
	s.log.add("update master data")
	s.masterData = &value
	return value, true, domain.OperationStats{}, nil
}

func (s *fakeStore) ListStaffQualifications(context.Context, int64) ([]domain.StaffQualification, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (s *fakeStore) FindStaffFinancialData(_ context.Context, staffID int64) (domain.StaffFinancialData, bool, domain.OperationStats, error) {
	if s.financial == nil || s.financial.StaffID != staffID {
		return domain.StaffFinancialData{}, false, domain.OperationStats{}, nil
	}
	return *s.financial, true, domain.OperationStats{}, nil
}

func (s *fakeStore) CreateStaffFinancialData(_ context.Context, value domain.StaffFinancialData) (domain.StaffFinancialData, domain.OperationStats, error) {
	s.log.add("create financial data")
	s.financial = &value
	return value, domain.OperationStats{}, nil
}

func (s *fakeStore) UpdateStaffFinancialData(_ context.Context, value domain.StaffFinancialData) (domain.StaffFinancialData, bool, domain.OperationStats, error) {
	s.log.add("update financial data")
	s.financial = &value
	return value, true, domain.OperationStats{}, nil
}

func (s *fakeStore) FindStaffDocument(_ context.Context, staffID, documentID int64, includeDeleted bool) (domain.StaffDocument, bool, domain.OperationStats, error) {
	for _, document := range s.documents {
		if document.StaffID == staffID && document.ID == documentID && (includeDeleted || document.DeletedAt == nil) {
			return document, true, domain.OperationStats{}, nil
		}
	}
	return domain.StaffDocument{}, false, domain.OperationStats{}, nil
}

func (s *fakeStore) CreateStaffDocument(_ context.Context, value domain.StaffDocument) (domain.StaffDocument, domain.OperationStats, error) {
	s.log.add("create document")
	value.ID = int64(len(s.documents) + 1)
	value.CreatedAt = time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	s.documents = append(s.documents, value)
	return value, domain.OperationStats{}, nil
}

func (s *fakeStore) SoftDeleteStaffDocument(_ context.Context, id, _ int64, _ time.Time) (int64, domain.OperationStats, error) {
	s.log.add("soft delete document")
	for index := range s.documents {
		if s.documents[index].ID == id && s.documents[index].DeletedAt == nil {
			now := time.Now()
			s.documents[index].DeletedAt = &now
			return 1, domain.OperationStats{}, nil
		}
	}
	return 0, domain.OperationStats{}, nil
}

func (s *fakeStore) CompleteStaffDocumentFileCleanupByFilename(context.Context, string, time.Time) (domain.OperationStats, error) {
	s.log.add("complete cleanup intent")
	return domain.OperationStats{}, nil
}

func (s *fakeStore) QueueStaffDocumentFileCleanup(_ context.Context, value domain.StaffDocumentFileCleanup) (domain.OperationStats, error) {
	s.queued = append(s.queued, value)
	return domain.OperationStats{}, nil
}

func (s *fakeStore) ListStaffDocuments(_ context.Context, filter domain.StaffDocumentFilter) ([]domain.StaffDocument, domain.OperationStats, error) {
	s.log.add("list documents")
	var result []domain.StaffDocument
	for _, document := range s.documents {
		if filter.StaffID == 0 || document.StaffID == filter.StaffID {
			result = append(result, document)
		}
	}
	return result, domain.OperationStats{}, nil
}

// fakeTransaction joins a nested unit of work to the ambient one, like the
// tenant runtime does: only the outermost call begins and ends.
type fakeTransaction struct {
	log   *eventLog
	depth *int
}

func (t fakeTransaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if *t.depth == 0 {
		t.log.add("begin")
	}
	*t.depth++
	err := callback(ctx)
	*t.depth--
	if *t.depth == 0 {
		t.log.add("end")
	}
	return err
}
func (fakeTransaction) LockStaffQualifications(context.Context, int64) error { return nil }
func (fakeTransaction) LockStaffBalance(context.Context, int64) error        { return nil }
func (fakeTransaction) LockStaffAbsence(context.Context, int64) error        { return nil }
func (fakeTransaction) LockStaffShifts(context.Context, int64) error         { return nil }

type fakeSubjects struct {
	log             *eventLog
	subject         domain.StaffSubject
	missing         bool
	personnelTaken  bool
	offboarded      []int64
	offboardedCalls int
}

func (s *fakeSubjects) StaffWithPerson(context.Context, int64) (domain.StaffSubject, error) {
	if s.missing {
		return domain.StaffSubject{}, &domain.RecordNotFoundError{Op: "find staff"}
	}
	return s.subject, nil
}

func (s *fakeSubjects) StaffExists(context.Context, int64) error {
	if s.missing {
		return &domain.RecordNotFoundError{Op: "find staff"}
	}
	return nil
}

func (s *fakeSubjects) LockStaff(context.Context, int64, bool) (domain.StaffSubject, error) {
	s.log.add("lock staff")
	if s.missing {
		return domain.StaffSubject{}, &domain.RecordNotFoundError{Op: "find staff"}
	}
	return s.subject, nil
}

func (s *fakeSubjects) UpdatePerson(context.Context, int64, string, string, *string) error {
	s.log.add("update person")
	return nil
}

func (s *fakeSubjects) SetEmploymentType(context.Context, int64, *string) error {
	s.log.add("set employment type")
	return nil
}

func (s *fakeSubjects) SetPersonnelNumber(context.Context, int64, *string) error {
	s.log.add("set personnel number")
	if s.personnelTaken {
		return domain.ErrStaffPersonnelNumberTaken
	}
	return nil
}

func (s *fakeSubjects) OffboardedStaffIDs(context.Context) ([]int64, error) {
	s.offboardedCalls++
	return s.offboarded, nil
}

type fakeAudit struct {
	log           *eventLog
	failing       bool
	masterData    []domain.MasterDataChange
	personnel     []domain.PersonnelNumberChange
	dataAccess    []domain.DataAccess
	failAccessLog bool
}

func (a *fakeAudit) RecordMasterDataChange(_ context.Context, change domain.MasterDataChange) error {
	a.log.add("audit " + change.Section + "/" + change.Field)
	if a.failing {
		return errAuditDown
	}
	a.masterData = append(a.masterData, change)
	return nil
}

func (a *fakeAudit) RecordPersonnelNumberChange(_ context.Context, change domain.PersonnelNumberChange) error {
	a.log.add("audit personnel number")
	if a.failing {
		return errAuditDown
	}
	a.personnel = append(a.personnel, change)
	return nil
}

func (a *fakeAudit) RecordDataAccess(_ context.Context, access domain.DataAccess) error {
	a.log.add("access log " + access.Resource)
	if a.failing || a.failAccessLog {
		return errAuditDown
	}
	a.dataAccess = append(a.dataAccess, access)
	return nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }
func (c fixedClock) Today() string  { return c.now.Format(domain.DateLayout) }

type adminFixture struct {
	admin    *StaffAdmin
	log      *eventLog
	store    *fakeStore
	subjects *fakeSubjects
	audit    *fakeAudit
	warnings []string
	now      time.Time
}

func newAdminFixture() *adminFixture {
	log := &eventLog{}
	fixture := &adminFixture{
		log:      log,
		store:    &fakeStore{log: log},
		subjects: &fakeSubjects{log: log, subject: domain.StaffSubject{ID: 7, PersonID: 70, FirstName: "Ada", LastName: "L", EmploymentType: ptr(domain.EmploymentTypeFullTime)}},
		audit:    &fakeAudit{log: log},
		now:      time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC),
	}
	unit := fakeTransaction{log: log, depth: new(int)}
	clock := fixedClock{now: fixture.now}
	records := NewRecords(fixture.store, unit, clock, func(ports.Observation) {})
	fixture.admin = NewStaffAdmin(records, unit, fixture.subjects, fixture.audit, domain.DocumentAuthority{
		Health: "health", Financial: "financial", General: "general",
		Allows: func(required string, held []string) bool {
			for _, permission := range held {
				if permission == required {
					return true
				}
			}
			return false
		},
	}, clock, func(message string, _ ...any) { fixture.warnings = append(fixture.warnings, message) })
	return fixture
}

func ptr[T any](value T) *T { return &value }

func TestPersonnelNumberChangeIsAuditedBeforeItIsWritten(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	staff, err := f.admin.UpdatePersonnelNumber(t.Context(), 7, ptr(" 4711 "), 3, "  neu vergeben ")
	require.NoError(t, err)

	assert.Equal(t, []string{"begin", "lock staff", "audit personnel number", "set personnel number", "end"}, f.log.events)
	assert.Equal(t, "4711", *staff.PersonnelNumber)
	require.Len(t, f.audit.personnel, 1)
	assert.Equal(t, domain.PersonnelNumberChange{StaffID: 7, ChangedBy: 3, OldValue: "", NewValue: "4711", Note: "neu vergeben"}, f.audit.personnel[0])
}

func TestPersonnelNumberNoOpWritesNoAuditRow(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	f.subjects.subject.PersonnelNumber = ptr("4711")
	_, err := f.admin.UpdatePersonnelNumber(t.Context(), 7, ptr("4711"), 3, "")
	require.NoError(t, err)
	assert.Empty(t, f.audit.personnel)
	assert.NotContains(t, f.log.events, "set personnel number")
}

func TestPersonnelNumberRefusesWithoutTraceOrWithBadInput(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	_, err := f.admin.UpdatePersonnelNumber(t.Context(), 7, ptr("4711"), 0, "")
	require.Error(t, err, "the actor is required")
	_, err = f.admin.UpdatePersonnelNumber(t.Context(), 7, ptr("A1"), 3, "")
	require.ErrorIs(t, err, domain.ErrPersonnelNumberInvalid)
	assert.Empty(t, f.log.events, "invalid input never opens a transaction")

	f.audit.failing = true
	_, err = f.admin.UpdatePersonnelNumber(t.Context(), 7, ptr("4711"), 3, "")
	require.ErrorIs(t, err, errAuditDown)
	assert.NotContains(t, f.log.events, "set personnel number", "no change without a trace")

	f = newAdminFixture()
	f.subjects.personnelTaken = true
	_, err = f.admin.UpdatePersonnelNumber(t.Context(), 7, ptr("4711"), 3, "")
	require.ErrorIs(t, err, domain.ErrStaffPersonnelNumberTaken)
}

func TestSectionWriteRecordsEveryFieldBeforeApplying(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	err := f.admin.UpdateStaffStammdatenPerson(t.Context(), 7, domain.PersonSection{
		FirstName: "Ada", LastName: "Lovelace", Gender: ptr(domain.GenderFemale),
	}, 3, "Ersteinrichtung")
	require.NoError(t, err)

	assert.Equal(t, []string{
		"begin", "lock staff", "audit person/last_name", "audit person/gender",
		"update person", "create master data", "end",
	}, f.log.events)
	require.Len(t, f.audit.masterData, 2)
	assert.Equal(t, "Ersteinrichtung", f.audit.masterData[0].Note)
	assert.Equal(t, domain.GenderFemale, *f.store.masterData.Gender)
}

func TestSectionWriteWithoutChangesTouchesNothing(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	err := f.admin.UpdateStaffStammdatenPerson(t.Context(), 7, domain.PersonSection{FirstName: "Ada", LastName: "L"}, 3, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"begin", "lock staff", "end"}, f.log.events)
	assert.Empty(t, f.audit.masterData)
}

func TestSectionWriteStopsWhenTheAuditFails(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	f.audit.failing = true
	err := f.admin.UpdateStaffStammdatenKontakt(t.Context(), 7, domain.ContactSection{Phone: ptr("0123")}, 3, "")
	require.ErrorIs(t, err, errAuditDown)
	assert.Contains(t, err.Error(), "write stammdaten audit")
	assert.NotContains(t, f.log.events, "create master data")
	assert.Nil(t, f.store.masterData, "nothing is applied after a failed audit")

	err = f.admin.UpdateStaffStammdatenKontakt(t.Context(), 7, domain.ContactSection{}, 0, "")
	require.Error(t, err, "the actor is required")
}

func TestContractSectionWritesEmploymentTypeOnlyWhenItChanged(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	err := f.admin.UpdateStaffStammdatenArbeitsvertrag(t.Context(), 7, domain.ContractSection{
		EmploymentType: ptr(domain.EmploymentTypeFullTime), WeeklyHours: ptr(30.0),
	}, 3, "")
	require.NoError(t, err)
	assert.NotContains(t, f.log.events, "set employment type", "the type did not change")
	assert.Contains(t, f.log.events, "create master data")

	f = newAdminFixture()
	err = f.admin.UpdateStaffStammdatenArbeitsvertrag(t.Context(), 7, domain.ContractSection{EmploymentType: ptr(domain.EmploymentTypePartTime)}, 3, "")
	require.NoError(t, err)
	assert.Contains(t, f.log.events, "set employment type")
}

func TestFinancialUpdateAuditsMaskedValuesOnly(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	err := f.admin.UpdateStaffFinancial(t.Context(), 7, domain.FinancialSection{IBAN: ptr("DE89 3704 0044 0532 0130 00")}, 3, "")
	require.NoError(t, err)
	require.Len(t, f.audit.masterData, 1)
	assert.Equal(t, domain.AuditSectionBankSteuer, f.audit.masterData[0].Section)
	assert.Equal(t, "•••• 3000", f.audit.masterData[0].NewValue)
	assert.Equal(t, "DE89370400440532013000", *f.store.financial.IBAN, "the store keeps the normalized plaintext")
	assert.Contains(t, f.log.events, "create financial data")
}

func TestFinancialReadIsLoggedBeforeAnyValueIsServed(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	f.store.financial = &domain.StaffFinancialData{StaffID: 7, IBAN: ptr("DE89370400440532013000")}

	masked, err := f.admin.StaffFinancialMasked(t.Context(), 7, 9, "  ")
	require.NoError(t, err)
	assert.Equal(t, "•••• 3000", *masked.IBANMasked)
	require.Len(t, f.audit.dataAccess, 1)
	access := f.audit.dataAccess[0]
	assert.Equal(t, domain.DataAccessFinancialView, access.Resource)
	assert.Equal(t, "unknown", access.ActorRole, "a blank role is logged as unknown")
	assert.Equal(t, f.now, access.At)

	plain, err := f.admin.RevealStaffFinancial(t.Context(), 7, 9, "admin")
	require.NoError(t, err)
	assert.Equal(t, "DE89370400440532013000", *plain.IBAN)
	assert.Equal(t, domain.DataAccessFinancialReveal, f.audit.dataAccess[1].Resource)

	f.audit.failAccessLog = true
	plain, err = f.admin.RevealStaffFinancial(t.Context(), 7, 9, "admin")
	require.ErrorIs(t, err, errAuditDown)
	assert.Nil(t, plain.IBAN, "no value leaves when the access log write fails")

	_, err = f.admin.StaffFinancialMasked(t.Context(), 7, 0, "admin")
	require.Error(t, err, "the actor is required")

	f.subjects.missing = true
	_, err = f.admin.StaffFinancialMasked(t.Context(), 8, 9, "admin")
	require.Error(t, err, "an unknown staff member discloses nothing")
	assert.Len(t, f.audit.dataAccess, 2)
}

func TestReadWithoutStoredFinancialDataServesAnEmptyRecord(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	masked, err := f.admin.StaffFinancialMasked(t.Context(), 7, 9, "admin")
	require.NoError(t, err)
	assert.Equal(t, domain.StaffFinancialMasked{StaffID: 7}, masked)
	plain, err := f.admin.RevealStaffFinancial(t.Context(), 7, 9, "admin")
	require.NoError(t, err)
	assert.Equal(t, domain.StaffFinancialData{StaffID: 7}, plain)
}

func documentInput(category string) CreateDocumentInput {
	return CreateDocumentInput{StaffID: 7, Category: category, FilenameDisplay: " Vertrag.pdf ", FilenameStored: "abc.pdf", SizeBytes: 10, ContentType: "application/pdf"}
}

func TestCreateDocumentAuditsThenStoresThenSettlesTheCleanupIntent(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	actor := domain.DocumentActor{AccountID: 4, Role: "hr", Permissions: []string{"general"}}
	info, err := f.admin.CreateStaffDocument(t.Context(), documentInput(domain.StaffDocumentCategoryArbeitsvertrag), actor)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"begin", "audit dokumente/arbeitsvertrag", "create document", "complete cleanup intent", "end",
	}, f.log.events)
	assert.Equal(t, "Vertrag.pdf", info.Document.FilenameDisplay)
	assert.Equal(t, actor.AccountID, info.Document.UploadedBy)
	assert.Empty(t, info.RetainUntil, "no contract end, so retention stays open")
	assert.Empty(t, f.warnings)
}

func TestCreateDocumentRefusesWhatTheActorMayNotTouch(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	hr := domain.DocumentActor{AccountID: 4, Permissions: []string{"general"}}

	_, err := f.admin.CreateStaffDocument(t.Context(), documentInput(domain.StaffDocumentCategoryAUBescheinigung), hr)
	require.ErrorIs(t, err, domain.ErrStaffDocumentForbidden)
	_, err = f.admin.CreateStaffDocument(t.Context(), documentInput("geheim"), hr)
	require.ErrorIs(t, err, domain.ErrStaffDocumentInvalid)
	blank := documentInput(domain.StaffDocumentCategoryZeugnis)
	blank.FilenameDisplay = "  "
	_, err = f.admin.CreateStaffDocument(t.Context(), blank, hr)
	require.ErrorIs(t, err, domain.ErrStaffDocumentInvalid)
	_, err = f.admin.CreateStaffDocument(t.Context(), documentInput(domain.StaffDocumentCategoryZeugnis), domain.DocumentActor{Permissions: []string{"general"}})
	require.Error(t, err, "the actor is required")
	assert.Empty(t, f.log.events, "a refused upload never opens a transaction")

	f.audit.failing = true
	_, err = f.admin.CreateStaffDocument(t.Context(), documentInput(domain.StaffDocumentCategoryZeugnis), hr)
	require.ErrorIs(t, err, errAuditDown)
	assert.Empty(t, f.store.documents, "no metadata row after a failed audit")
}

func TestSensitiveDownloadIsLoggedAndRefusedWhenTheLogFails(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	f.store.documents = []domain.StaffDocument{
		{ID: 1, StaffID: 7, Category: domain.StaffDocumentCategoryLohnabrechnung, FilenameDisplay: "Juli.pdf"},
		{ID: 2, StaffID: 7, Category: domain.StaffDocumentCategoryZeugnis, FilenameDisplay: "Zeugnis.pdf"},
	}
	payroll := domain.DocumentActor{AccountID: 4, Role: "payroll", Permissions: []string{"financial", "general"}}

	document, err := f.admin.ResolveStaffDocumentDownload(t.Context(), 7, 1, payroll)
	require.NoError(t, err)
	assert.Equal(t, "Juli.pdf", document.FilenameDisplay)
	require.Len(t, f.audit.dataAccess, 1)
	assert.Equal(t, domain.DataAccess{
		ActorAccountID: 4, ActorRole: "payroll", Resource: domain.DataAccessDocumentDownload, StaffID: 7,
		DocumentID: 1, Category: domain.StaffDocumentCategoryLohnabrechnung, At: f.now,
	}, f.audit.dataAccess[0])

	_, err = f.admin.ResolveStaffDocumentDownload(t.Context(), 7, 2, payroll)
	require.NoError(t, err)
	assert.Len(t, f.audit.dataAccess, 1, "a general document needs no access log row")

	_, err = f.admin.ResolveStaffDocumentDownload(t.Context(), 7, 1, domain.DocumentActor{AccountID: 4, Permissions: []string{"general"}})
	require.ErrorIs(t, err, domain.ErrStaffDocumentForbidden)

	f.audit.failAccessLog = true
	document, err = f.admin.ResolveStaffDocumentDownload(t.Context(), 7, 1, payroll)
	require.ErrorIs(t, err, errAuditDown)
	assert.Zero(t, document.ID, "no file is served when the access log write fails")

	_, err = f.admin.ResolveStaffDocumentDownload(t.Context(), 7, 99, payroll)
	var missing *domain.RecordNotFoundError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "find staff document", missing.Op)
}

func TestDeleteDocumentSoftDeletesThenAudits(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	f.store.documents = []domain.StaffDocument{{ID: 1, StaffID: 7, Category: domain.StaffDocumentCategoryZeugnis, FilenameDisplay: "Zeugnis.pdf"}}
	hr := domain.DocumentActor{AccountID: 4, Permissions: []string{"general"}}

	deleted, err := f.admin.DeleteStaffDocument(t.Context(), 7, 1, hr)
	require.NoError(t, err)
	require.NotNil(t, deleted.DeletedAt)
	assert.Equal(t, hr.AccountID, *deleted.DeletedBy)
	assert.Equal(t, []string{"begin", "soft delete document", "audit dokumente/zeugnis", "end"}, f.log.events)
	assert.Equal(t, "Zeugnis.pdf", f.audit.masterData[0].OldValue)
	assert.Empty(t, f.audit.masterData[0].NewValue)

	_, err = f.admin.DeleteStaffDocument(t.Context(), 7, 1, hr)
	var missing *domain.RecordNotFoundError
	require.ErrorAs(t, err, &missing, "a deleted document cannot be deleted again")
}

func TestListDocumentsFiltersByTheActorsCategories(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	f.store.documents = []domain.StaffDocument{{ID: 1, StaffID: 7, Category: domain.StaffDocumentCategoryZeugnis}}
	hr := domain.DocumentActor{AccountID: 4, Permissions: []string{"general"}}

	infos, visible, err := f.admin.ListStaffDocuments(t.Context(), 7, "", hr)
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.NotContains(t, visible, domain.StaffDocumentCategoryAUBescheinigung)

	_, _, err = f.admin.ListStaffDocuments(t.Context(), 7, domain.StaffDocumentCategoryAUBescheinigung, hr)
	require.ErrorIs(t, err, domain.ErrStaffDocumentForbidden)
	_, _, err = f.admin.ListStaffDocuments(t.Context(), 7, "geheim", hr)
	require.ErrorIs(t, err, domain.ErrStaffDocumentInvalid)

	f.log.events = nil
	infos, visible, err = f.admin.ListStaffDocuments(t.Context(), 7, "", domain.DocumentActor{AccountID: 4})
	require.NoError(t, err)
	assert.Empty(t, infos)
	assert.Empty(t, visible)
	assert.NotContains(t, f.log.events, "list documents", "an actor without any category never queries")
}

func TestQueueCleanupWaitsPastTheUploadDeadline(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	require.NoError(t, f.admin.QueueStaffDocumentFileCleanup(t.Context(), 7, "abc.pdf"))
	require.Len(t, f.store.queued, 1)
	assert.Equal(t, f.now.Add(domain.StaffDocumentCleanupDelay), f.store.queued[0].RetryAfter)
	assert.True(t, f.store.queued[0].RetryAfter.After(f.now.Add(domain.StaffDocumentUploadDeadline)))

	require.ErrorIs(t, f.admin.QueueStaffDocumentFileCleanup(t.Context(), 0, "abc.pdf"), domain.ErrStaffDocumentInvalid)
	require.ErrorIs(t, f.admin.QueueStaffDocumentFileCleanup(t.Context(), 7, "  "), domain.ErrStaffDocumentInvalid)
}

func TestOffboardedCleanupAsksTheMembershipOwner(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	documents, err := f.admin.ListOffboardedStaffDocumentsPendingFileCleanup(t.Context())
	require.NoError(t, err)
	assert.Empty(t, documents)
	assert.NotContains(t, f.log.events, "list documents", "nobody offboarded, nothing to query")

	f.subjects.offboarded = []int64{7}
	f.store.documents = []domain.StaffDocument{{ID: 1, StaffID: 7}}
	documents, err = f.admin.ListOffboardedStaffDocumentsPendingFileCleanup(t.Context())
	require.NoError(t, err)
	assert.Len(t, documents, 1)
	assert.Equal(t, 2, f.subjects.offboardedCalls)
}

func TestStammdatenReadsThroughTheSubjectPort(t *testing.T) {
	t.Parallel()

	f := newAdminFixture()
	data, err := f.admin.StaffStammdaten(t.Context(), 7)
	require.NoError(t, err)
	assert.Equal(t, f.subjects.subject.ID, data.Staff.ID)
	assert.Nil(t, data.MasterData, "no master data before the first section write")

	f.subjects.missing = true
	_, err = f.admin.StaffStammdaten(t.Context(), 7)
	var missing *domain.RecordNotFoundError
	require.ErrorAs(t, err, &missing)
}
