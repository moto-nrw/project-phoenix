package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// staffSubjects binds the public port to the application's.
type staffSubjects struct{ port workforce.StaffAdminSubjects }

func (s staffSubjects) StaffWithPerson(ctx context.Context, staffID int64) (domain.StaffSubject, error) {
	profile, err := s.port.StaffWithPerson(ctx, staffID)
	return domain.StaffSubject(profile), err
}

func (s staffSubjects) StaffExists(ctx context.Context, staffID int64) error {
	return s.port.StaffExists(ctx, staffID)
}

func (s staffSubjects) LockStaff(ctx context.Context, staffID int64, withPerson bool) (domain.StaffSubject, error) {
	profile, err := s.port.LockStaff(ctx, staffID, withPerson)
	return domain.StaffSubject(profile), err
}

func (s staffSubjects) UpdatePerson(ctx context.Context, personID int64, firstName, lastName string, birthday *string) error {
	return s.port.UpdatePerson(ctx, personID, firstName, lastName, birthday)
}

func (s staffSubjects) SetEmploymentType(ctx context.Context, staffID int64, value *string) error {
	return s.port.SetEmploymentType(ctx, staffID, value)
}

func (s staffSubjects) SetPersonnelNumber(ctx context.Context, staffID int64, value *string) error {
	err := s.port.SetPersonnelNumber(ctx, staffID, value)
	if errors.Is(err, workforce.ErrPersonnelNumberTaken) {
		return domain.ErrStaffPersonnelNumberTaken
	}
	return err
}

func (s staffSubjects) OffboardedStaffIDs(ctx context.Context) ([]int64, error) {
	return s.port.OffboardedStaffIDs(ctx)
}

var _ ports.StaffSubjects = staffSubjects{}

// staffAudit binds the public audit port to the application's.
type staffAudit struct{ port workforce.StaffAdminAudit }

func (a staffAudit) RecordMasterDataChange(ctx context.Context, change domain.MasterDataChange) error {
	return a.port.RecordMasterDataChange(ctx, workforce.StaffMasterDataChange(change))
}

func (a staffAudit) RecordPersonnelNumberChange(ctx context.Context, change domain.PersonnelNumberChange) error {
	return a.port.RecordPersonnelNumberChange(ctx, workforce.PersonnelNumberChange(change))
}

func (a staffAudit) RecordDataAccess(ctx context.Context, access domain.DataAccess) error {
	return a.port.RecordDataAccess(ctx, workforce.StaffDataAccess(access))
}

var _ ports.StaffAdminAudit = staffAudit{}

// StaffAdminEngine adapts the application to the public contract: it converts
// values and classifies failures, keeping the wording of the failure the HTTP
// layer renders.
// NewStaffAdminEngine serves the staff administration over admin and the
// plain record reads over records.
func NewStaffAdminEngine(admin *StaffAdmin, records *Service) StaffAdminEngine {
	return StaffAdminEngine{admin: admin, records: records}
}

// NewStaffSubjects adapts the root's staff subjects to the module's port.
func NewStaffSubjects(port workforce.StaffAdminSubjects) ports.StaffSubjects {
	return staffSubjects{port: port}
}

// NewStaffAudit adapts the root's staff audit to the module's port.
func NewStaffAudit(port workforce.StaffAdminAudit) ports.StaffAdminAudit {
	return staffAudit{port: port}
}

type StaffAdminEngine struct {
	admin *StaffAdmin
	// records serves the operations that are plain reads of the Workforce-owned
	// rows: the administration adds nothing to them.
	records *Service
}

func (e StaffAdminEngine) UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*workforce.StaffProfile, error) {
	subject, err := e.admin.UpdatePersonnelNumber(ctx, staffID, value, changedByStaffID, note)
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	return new(workforce.StaffProfile(subject)), nil
}

func (e StaffAdminEngine) StaffStammdaten(ctx context.Context, staffID int64) (*workforce.StaffStammdaten, error) {
	data, err := e.admin.StaffStammdaten(ctx, staffID)
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	result := &workforce.StaffStammdaten{Staff: workforce.StaffProfile(data.Staff), Qualifications: []workforce.StaffQualification{}}
	if data.MasterData != nil {
		result.MasterData = new(masterDataToPublic(*data.MasterData))
	}
	result.Qualifications = append(result.Qualifications, qualificationsToPublic(data.Qualifications)...)
	return result, nil
}

func (e StaffAdminEngine) UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input workforce.StammdatenPersonInput, changedByStaffID int64, note string) error {
	return MapStaffAdminError(e.admin.UpdateStaffStammdatenPerson(ctx, staffID, domain.PersonSection(input), changedByStaffID, note))
}

func (e StaffAdminEngine) UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input workforce.StammdatenKontaktInput, changedByStaffID int64, note string) error {
	return MapStaffAdminError(e.admin.UpdateStaffStammdatenKontakt(ctx, staffID, domain.ContactSection(input), changedByStaffID, note))
}

func (e StaffAdminEngine) UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input workforce.StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error {
	return MapStaffAdminError(e.admin.UpdateStaffStammdatenArbeitsvertrag(ctx, staffID, domain.ContractSection(input), changedByStaffID, note))
}

func (e StaffAdminEngine) ReplaceStaffQualificationList(ctx context.Context, staffID int64, inputs []workforce.StammdatenQualificationInput, changedByStaffID int64, note string) error {
	rows := make([]domain.QualificationInput, 0, len(inputs))
	for _, input := range inputs {
		rows = append(rows, domain.QualificationInput(input))
	}
	return MapStaffAdminError(e.admin.ReplaceStaffQualifications(ctx, staffID, rows, changedByStaffID, note))
}

func (e StaffAdminEngine) StaffFinancialMasked(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*workforce.StaffFinancialMasked, error) {
	masked, err := e.admin.StaffFinancialMasked(ctx, staffID, actorAccountID, actorRole)
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	return new(workforce.StaffFinancialMasked(masked)), nil
}

func (e StaffAdminEngine) RevealStaffFinancial(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*workforce.StaffFinancialPlain, error) {
	data, err := e.admin.RevealStaffFinancial(ctx, staffID, actorAccountID, actorRole)
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	return &workforce.StaffFinancialPlain{StaffID: staffID, IBAN: data.IBAN, TaxID: data.TaxID, SocialSecurityNumber: data.SocialSecurityNumber}, nil
}

func (e StaffAdminEngine) UpdateStaffFinancial(ctx context.Context, staffID int64, input workforce.StammdatenFinancialInput, changedByAccountID int64, note string) error {
	return MapStaffAdminError(e.admin.UpdateStaffFinancial(ctx, staffID, domain.FinancialSection(input), changedByAccountID, note))
}

func (e StaffAdminEngine) ListStaffDocuments(ctx context.Context, staffID int64, category string, actor workforce.StaffDocumentActor) ([]workforce.StaffDocumentInfo, []string, error) {
	infos, categories, err := e.admin.ListStaffDocuments(ctx, staffID, category, domain.DocumentActor(actor))
	if err != nil {
		return nil, nil, MapStaffAdminError(err)
	}
	result := make([]workforce.StaffDocumentInfo, 0, len(infos))
	for _, info := range infos {
		result = append(result, documentInfoToPublic(info))
	}
	return result, categories, nil
}

func (e StaffAdminEngine) CreateStaffDocument(ctx context.Context, input workforce.CreateStaffDocumentInput, actor workforce.StaffDocumentActor) (*workforce.StaffDocumentInfo, error) {
	info, err := e.admin.CreateStaffDocument(ctx, CreateDocumentInput(input), domain.DocumentActor(actor))
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	return new(documentInfoToPublic(info)), nil
}

func (e StaffAdminEngine) ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor workforce.StaffDocumentActor) (*workforce.StaffDocument, error) {
	return publicDocument(e.admin.ResolveStaffDocumentDownload(ctx, staffID, documentID, domain.DocumentActor(actor)))
}

func (e StaffAdminEngine) DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor workforce.StaffDocumentActor) (*workforce.StaffDocument, error) {
	return publicDocument(e.admin.DeleteStaffDocument(ctx, staffID, documentID, domain.DocumentActor(actor)))
}

func (e StaffAdminEngine) ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor workforce.StaffDocumentActor) (*workforce.StaffDocument, error) {
	return publicDocument(e.admin.ResolveStaffDocumentCleanup(ctx, staffID, documentID, domain.DocumentActor(actor)))
}

func (e StaffAdminEngine) ListStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64) ([]workforce.StaffDocument, error) {
	return publicRecordDocuments(e.records.ListStaffDocuments(ctx, workforce.StaffDocumentFilter(domain.PendingFileCleanupFilter(staffID))))
}

func (e StaffAdminEngine) ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]workforce.StaffDocument, error) {
	return publicDocuments(e.admin.ListOffboardedStaffDocumentsPendingFileCleanup(ctx))
}

func (e StaffAdminEngine) ListDeletedStaffDocumentsPendingFileCleanups(ctx context.Context) ([]workforce.StaffDocument, error) {
	return publicRecordDocuments(e.records.ListStaffDocuments(ctx, workforce.StaffDocumentFilter(domain.DeletedPendingFileCleanupFilter(0, nil))))
}

func (e StaffAdminEngine) ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor workforce.StaffDocumentActor) ([]workforce.StaffDocument, error) {
	return publicDocuments(e.admin.ListDeletedStaffDocumentsPendingFileCleanup(ctx, staffID, domain.DocumentActor(actor)))
}

func (e StaffAdminEngine) MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error {
	return MapStaffAdminError(e.admin.MarkStaffDocumentFileDeleted(ctx, documentID))
}

func (e StaffAdminEngine) QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error {
	return MapStaffAdminError(e.admin.QueueStaffDocumentFileCleanup(ctx, staffID, storedName))
}

func (e StaffAdminEngine) ListQueuedStaffDocumentFileCleanup(ctx context.Context, staffID int64) ([]workforce.StaffDocumentFileCleanup, error) {
	return publicRecordCleanups(e.records.ListQueuedStaffDocumentFileCleanups(ctx, staffID))
}

func (e StaffAdminEngine) ListQueuedStaffDocumentFileCleanups(ctx context.Context) ([]workforce.StaffDocumentFileCleanup, error) {
	return publicRecordCleanups(e.records.ListQueuedStaffDocumentFileCleanups(ctx, 0))
}

func (e StaffAdminEngine) MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	return MapStaffAdminError(e.admin.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanupID))
}

func (e StaffAdminEngine) MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return MapStaffAdminError(e.admin.MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx, storedName))
}

func (e StaffAdminEngine) ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error {
	return MapStaffAdminError(e.admin.ActivateQueuedStaffDocumentFileCleanup(ctx, storedName))
}

func documentInfoToPublic(info domain.StaffDocumentInfo) workforce.StaffDocumentInfo {
	return workforce.StaffDocumentInfo{Document: documentToPublic(info.Document), RetainUntil: info.RetainUntil, ReviewDue: info.ReviewDue}
}

func publicDocument(document domain.StaffDocument, err error) (*workforce.StaffDocument, error) {
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	return new(documentToPublic(document)), nil
}

func publicDocuments(documents []domain.StaffDocument, err error) ([]workforce.StaffDocument, error) {
	if err != nil {
		return nil, MapStaffAdminError(err)
	}
	result := make([]workforce.StaffDocument, 0, len(documents))
	for _, document := range documents {
		result = append(result, documentToPublic(document))
	}
	return result, nil
}

// missingRecord is a missing personnel record row that also answers to
// sql.ErrNoRows, the way the repositories always reported it.
type missingRecord struct{ *domain.RecordNotFoundError }

func (e missingRecord) Is(target error) bool {
	return target == transactionruntime.ErrNoRows || e.RecordNotFoundError.Is(target) || target == workforce.ErrStaffDocumentNotFound
}

func (e missingRecord) Unwrap() error { return e.RecordNotFoundError }

// mapStaffAdminError classifies a failure of the administration. The five
// caller-facing kinds keep the wording of the failure, which is what the HTTP
// layer renders; a missing row keeps its repository shape; every other failure
// takes the personnel record and work-time classification.
// MapStaffAdminError maps the staff administration errors onto the contract,
// keeping the wording the HTTP layer renders.
func MapStaffAdminError(err error) error {
	if err == nil {
		return nil
	}
	for _, pair := range []struct{ cause, kind error }{
		{domain.ErrStaffDocumentForbidden, workforce.ErrStaffDocumentForbidden},
		{domain.ErrStaffDocumentInvalid, workforce.ErrStaffDocumentInvalid},
		{domain.ErrStaffPersonnelNumberTaken, workforce.ErrPersonnelNumberTaken},
		{domain.ErrPersonnelNumberInvalid, workforce.ErrPersonnelNumberInvalid},
		{domain.ErrStaffStammdatenInvalid, workforce.ErrStaffStammdatenInvalid},
	} {
		if errors.Is(err, pair.cause) {
			return &workforce.StaffAdminError{Kind: pair.kind, Cause: err}
		}
	}
	if notFound, ok := errors.AsType[*domain.RecordNotFoundError](err); ok {
		return missingRecord{notFound}
	}
	return mapStaffRecordError(err)
}
