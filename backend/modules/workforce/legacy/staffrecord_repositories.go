package legacy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The adapters in this file keep the retained models/users personnel record
// contracts alive on top of the Workforce capability while their consumers
// migrate (#2690): Stammdaten, qualifications, bank and tax data, staff
// documents and their file cleanup intents. They perform no persistence.

// --- master data ---

type staffMasterDataRepository struct{ workforce workforce.Capability }

// NewStaffMasterDataRepository serves userModels.StaffMasterDataRepository
// from the Workforce capability.
func NewStaffMasterDataRepository(capability workforce.Capability) userModels.StaffMasterDataRepository {
	if capability == nil {
		panic("staff master data repository adapter: Workforce capability is required")
	}
	return staffMasterDataRepository{workforce: capability}
}

func (r staffMasterDataRepository) Create(ctx context.Context, data *userModels.StaffMasterData) error {
	if data == nil {
		return errors.New("StaffMasterData cannot be nil or zero value")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffMasterData(ctx, masterDataToCapability(data))
	if err != nil {
		return staffRecordWriteError("create", err)
	}
	applyMasterDataToLegacy(data, created)
	return nil
}

func (r staffMasterDataRepository) Update(ctx context.Context, data *userModels.StaffMasterData) error {
	if data == nil {
		return errors.New("StaffMasterData cannot be nil or zero value")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffMasterData(ctx, masterDataToCapability(data))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffMasterDataNotFound) {
			return rowsAffectedError("update StaffMasterData")
		}
		return staffRecordWriteError("update", err)
	}
	applyMasterDataToLegacy(data, updated)
	return nil
}

// FindByStaffID returns (nil, nil) when no row exists yet: an empty
// Stammdaten tab is a normal state, not an error.
func (r staffMasterDataRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.StaffMasterData, error) {
	value, err := r.workforce.FindStaffMasterData(ctx, staffID)
	if err != nil {
		if errors.Is(err, workforce.ErrStaffMasterDataNotFound) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find staff master data by staff id", Err: err}
	}
	entity := &userModels.StaffMasterData{}
	applyMasterDataToLegacy(entity, value)
	return entity, nil
}

// --- qualifications ---

type staffQualificationRepository struct{ workforce workforce.Capability }

// NewStaffQualificationRepository serves
// userModels.StaffQualificationRepository from the Workforce capability.
func NewStaffQualificationRepository(capability workforce.Capability) userModels.StaffQualificationRepository {
	if capability == nil {
		panic("staff qualification repository adapter: Workforce capability is required")
	}
	return staffQualificationRepository{workforce: capability}
}

func (r staffQualificationRepository) ListByStaffID(ctx context.Context, staffID int64) ([]*userModels.StaffQualification, error) {
	values, err := r.workforce.ListStaffQualifications(ctx, staffID)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff qualifications", Err: err}
	}
	result := make([]*userModels.StaffQualification, 0, len(values))
	for _, value := range values {
		entity := &userModels.StaffQualification{}
		applyQualificationToLegacy(entity, value)
		result = append(result, entity)
	}
	return result, nil
}

// ReplaceForStaff replaces the whole list in one unit of work and writes the
// stored identities back onto the caller's rows.
func (r staffQualificationRepository) ReplaceForStaff(ctx context.Context, staffID int64, qualifications []*userModels.StaffQualification) error {
	values := make([]workforce.StaffQualification, 0, len(qualifications))
	for _, qualification := range qualifications {
		if qualification == nil {
			return errors.New("invalid staff qualification: StaffQualification cannot be nil or zero value")
		}
		qualification.StaffID = staffID
		if err := qualification.Validate(); err != nil {
			return fmt.Errorf("invalid staff qualification: %w", err)
		}
		values = append(values, workforce.StaffQualification{
			ID: qualification.ID, TenantID: qualification.TenantID, StaffID: staffID, Name: qualification.Name,
			AcquiredOn: optionalDateString(qualification.AcquiredOn), ExpiresOn: optionalDateString(qualification.ExpiresOn),
		})
	}
	stored, err := r.workforce.ReplaceStaffQualifications(ctx, staffID, values)
	if err != nil {
		if errors.Is(err, workforce.ErrInvalidStaffRecord) {
			return fmt.Errorf("invalid staff qualification: %w", errors.New(err.Error()))
		}
		return &modelBase.DatabaseError{Op: "replace staff qualifications", Err: err}
	}
	for index, qualification := range qualifications {
		if index < len(stored) {
			applyQualificationToLegacy(qualification, stored[index])
		}
	}
	return nil
}

// --- financial data ---

type staffFinancialDataRepository struct{ workforce workforce.Capability }

// NewStaffFinancialDataRepository serves
// userModels.StaffFinancialDataRepository from the Workforce capability.
func NewStaffFinancialDataRepository(capability workforce.Capability) userModels.StaffFinancialDataRepository {
	if capability == nil {
		panic("staff financial data repository adapter: Workforce capability is required")
	}
	return staffFinancialDataRepository{workforce: capability}
}

func (r staffFinancialDataRepository) Create(ctx context.Context, data *userModels.StaffFinancialData) error {
	if data == nil {
		return errors.New("StaffFinancialData cannot be nil or zero value")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffFinancialData(ctx, financialToCapability(data))
	if err != nil {
		return staffRecordWriteError("create", err)
	}
	applyFinancialToLegacy(data, created)
	return nil
}

func (r staffFinancialDataRepository) Update(ctx context.Context, data *userModels.StaffFinancialData) error {
	if data == nil {
		return errors.New("StaffFinancialData cannot be nil or zero value")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffFinancialData(ctx, financialToCapability(data))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffFinancialDataNotFound) {
			return rowsAffectedError("update StaffFinancialData")
		}
		return staffRecordWriteError("update", err)
	}
	applyFinancialToLegacy(data, updated)
	return nil
}

func (r staffFinancialDataRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.StaffFinancialData, error) {
	value, err := r.workforce.FindStaffFinancialData(ctx, staffID)
	if err != nil {
		if errors.Is(err, workforce.ErrStaffFinancialDataNotFound) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find staff financial data by staff id", Err: err}
	}
	entity := &userModels.StaffFinancialData{}
	applyFinancialToLegacy(entity, value)
	return entity, nil
}

// --- documents ---

// StaffDocumentRepository is userModels.StaffDocumentRepository without the
// offboarding listing, which needs the offboarded staff from School
// Membership. The composition root completes that contract by resolving the
// staff through the owner and calling ListPendingFileCleanupsForStaff.
type StaffDocumentRepository interface {
	Create(ctx context.Context, doc *userModels.StaffDocument) error
	FindForStaff(ctx context.Context, staffID, documentID int64) (*userModels.StaffDocument, error)
	FindForStaffIncludingDeleted(ctx context.Context, staffID, documentID int64) (*userModels.StaffDocument, error)
	ListByStaffID(ctx context.Context, staffID int64, categories []string) ([]*userModels.StaffDocument, error)
	ListPendingFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*userModels.StaffDocument, error)
	// ListPendingFileCleanupsForStaff returns the documents of the given
	// staff members whose stored file still has to be removed.
	ListPendingFileCleanupsForStaff(ctx context.Context, staffIDs []int64) ([]*userModels.StaffDocument, error)
	ListDeletedPendingFileCleanups(ctx context.Context) ([]*userModels.StaffDocument, error)
	ListDeletedPendingFileCleanupByStaffID(ctx context.Context, staffID int64, categories []string) ([]*userModels.StaffDocument, error)
	SoftDelete(ctx context.Context, doc *userModels.StaffDocument, deletedBy int64) error
	MarkFileDeleted(ctx context.Context, documentID int64) error
	QueueFileCleanup(ctx context.Context, cleanup *userModels.StaffDocumentFileCleanup) error
	ListQueuedFileCleanups(ctx context.Context) ([]*userModels.StaffDocumentFileCleanup, error)
	ListQueuedFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*userModels.StaffDocumentFileCleanup, error)
	MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error
	ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error
}

type staffDocumentRepository struct{ workforce workforce.Capability }

// NewStaffDocumentRepository serves the staff document contract from the
// Workforce capability.
func NewStaffDocumentRepository(capability workforce.Capability) StaffDocumentRepository {
	if capability == nil {
		panic("staff document repository adapter: Workforce capability is required")
	}
	return staffDocumentRepository{workforce: capability}
}

func (r staffDocumentRepository) Create(ctx context.Context, doc *userModels.StaffDocument) error {
	if doc == nil {
		return errors.New("StaffDocument cannot be nil or zero value")
	}
	if err := doc.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffDocument(ctx, documentToCapability(doc))
	if err != nil {
		return staffRecordWriteError("create staff document", err)
	}
	applyDocumentToLegacy(doc, created)
	return nil
}

func (r staffDocumentRepository) FindForStaff(ctx context.Context, staffID, documentID int64) (*userModels.StaffDocument, error) {
	return r.find(ctx, staffID, documentID, false)
}

func (r staffDocumentRepository) FindForStaffIncludingDeleted(ctx context.Context, staffID, documentID int64) (*userModels.StaffDocument, error) {
	return r.find(ctx, staffID, documentID, true)
}

func (r staffDocumentRepository) find(ctx context.Context, staffID, documentID int64, includeDeleted bool) (*userModels.StaffDocument, error) {
	value, err := r.workforce.FindStaffDocument(ctx, staffID, documentID, includeDeleted)
	if err != nil {
		if errors.Is(err, workforce.ErrStaffDocumentNotFound) || errors.Is(err, workforce.ErrInvalidStaffRecord) {
			return nil, &modelBase.DatabaseError{Op: "find staff document", Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
		}
		return nil, &modelBase.DatabaseError{Op: "find staff document", Err: err}
	}
	return documentToLegacy(value), nil
}

func (r staffDocumentRepository) ListByStaffID(ctx context.Context, staffID int64, categories []string) ([]*userModels.StaffDocument, error) {
	if len(categories) == 0 {
		return []*userModels.StaffDocument{}, nil
	}
	return r.list(ctx, "list staff documents", workforce.StaffDocumentFilter{StaffID: staffID, Categories: categories})
}

func (r staffDocumentRepository) ListPendingFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*userModels.StaffDocument, error) {
	return r.list(ctx, "list pending staff document cleanup", workforce.StaffDocumentFilter{StaffID: staffID, IncludeDeleted: true, FilePending: true})
}

func (r staffDocumentRepository) ListPendingFileCleanupsForStaff(ctx context.Context, staffIDs []int64) ([]*userModels.StaffDocument, error) {
	if len(staffIDs) == 0 {
		return []*userModels.StaffDocument{}, nil
	}
	return r.list(ctx, "list offboarded staff document cleanup", workforce.StaffDocumentFilter{StaffIDs: staffIDs, IncludeDeleted: true, FilePending: true})
}

func (r staffDocumentRepository) ListDeletedPendingFileCleanups(ctx context.Context) ([]*userModels.StaffDocument, error) {
	return r.list(ctx, "list deleted staff document cleanup", workforce.StaffDocumentFilter{DeletedOnly: true, FilePending: true})
}

func (r staffDocumentRepository) ListDeletedPendingFileCleanupByStaffID(ctx context.Context, staffID int64, categories []string) ([]*userModels.StaffDocument, error) {
	if len(categories) == 0 {
		return []*userModels.StaffDocument{}, nil
	}
	return r.list(ctx, "list pending deleted staff document cleanup", workforce.StaffDocumentFilter{
		StaffID: staffID, Categories: categories, DeletedOnly: true, FilePending: true,
	})
}

func (r staffDocumentRepository) list(ctx context.Context, op string, filter workforce.StaffDocumentFilter) ([]*userModels.StaffDocument, error) {
	values, err := r.workforce.ListStaffDocuments(ctx, filter)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	result := make([]*userModels.StaffDocument, 0, len(values))
	for _, value := range values {
		result = append(result, documentToLegacy(value))
	}
	return result, nil
}

// SoftDelete stamps deleted_at and deleted_by in one update; a row that is
// already gone reports the not-found sentinel the service maps to 404.
func (r staffDocumentRepository) SoftDelete(ctx context.Context, doc *userModels.StaffDocument, deletedBy int64) error {
	if doc == nil {
		return errors.New("StaffDocument cannot be nil or zero value")
	}
	now := time.Now()
	affected, err := r.workforce.SoftDeleteStaffDocument(ctx, doc.ID, deletedBy, now)
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete staff document", Err: err}
	}
	if affected == 0 {
		return &modelBase.DatabaseError{Op: "soft delete staff document", Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	doc.DeletedAt = &now
	doc.DeletedBy = &deletedBy
	return nil
}

func (r staffDocumentRepository) MarkFileDeleted(ctx context.Context, documentID int64) error {
	if err := r.workforce.MarkStaffDocumentFileDeleted(ctx, documentID, time.Now()); err != nil {
		return &modelBase.DatabaseError{Op: "mark staff document file deleted", Err: err}
	}
	return nil
}

func (r staffDocumentRepository) QueueFileCleanup(ctx context.Context, cleanup *userModels.StaffDocumentFileCleanup) error {
	if cleanup == nil {
		return errors.New("StaffDocumentFileCleanup cannot be nil or zero value")
	}
	err := r.workforce.QueueStaffDocumentFileCleanup(ctx, workforce.StaffDocumentFileCleanup{
		ID: cleanup.ID, TenantID: cleanup.TenantID, StaffID: cleanup.StaffID, FilenameStored: cleanup.FilenameStored,
		RetryAfter: cleanup.RetryAfter, CleanedAt: cleanup.CleanedAt,
	})
	if err != nil {
		return staffRecordWriteError("queue staff document file cleanup", err)
	}
	return nil
}

func (r staffDocumentRepository) ListQueuedFileCleanups(ctx context.Context) ([]*userModels.StaffDocumentFileCleanup, error) {
	return r.listCleanups(ctx, 0)
}

func (r staffDocumentRepository) ListQueuedFileCleanupByStaffID(ctx context.Context, staffID int64) ([]*userModels.StaffDocumentFileCleanup, error) {
	return r.listCleanups(ctx, staffID)
}

func (r staffDocumentRepository) listCleanups(ctx context.Context, staffID int64) ([]*userModels.StaffDocumentFileCleanup, error) {
	values, err := r.workforce.ListQueuedStaffDocumentFileCleanups(ctx, staffID)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list queued staff document file cleanup", Err: err}
	}
	result := make([]*userModels.StaffDocumentFileCleanup, 0, len(values))
	for _, value := range values {
		entity := &userModels.StaffDocumentFileCleanup{
			StaffID: value.StaffID, FilenameStored: value.FilenameStored, RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt,
		}
		entity.ID = value.ID
		entity.CreatedAt = value.CreatedAt
		entity.UpdatedAt = value.UpdatedAt
		entity.TenantID = value.TenantID
		result = append(result, entity)
	}
	return result, nil
}

func (r staffDocumentRepository) MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	if err := r.workforce.CompleteStaffDocumentFileCleanup(ctx, cleanupID); err != nil {
		return &modelBase.DatabaseError{Op: "mark queued staff document file cleanup complete", Err: err}
	}
	return nil
}

func (r staffDocumentRepository) MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error {
	if err := r.workforce.CompleteStaffDocumentFileCleanupByFilename(ctx, filename); err != nil {
		return &modelBase.DatabaseError{Op: "mark queued staff document file cleanup complete", Err: err}
	}
	return nil
}

func (r staffDocumentRepository) ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error {
	if err := r.workforce.ActivateStaffDocumentFileCleanup(ctx, filename); err != nil {
		return &modelBase.DatabaseError{Op: "activate queued staff document file cleanup", Err: err}
	}
	return nil
}

// --- shared helpers ---

// staffRecordWriteError keeps the legacy write error shape: validation
// failures surface bare, everything else is a DatabaseError.
func staffRecordWriteError(op string, err error) error {
	if errors.Is(err, workforce.ErrInvalidStaffRecord) {
		return errors.New(err.Error())
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}

func optionalDateString(value *timezone.Date) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func optionalDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	date := timezone.Date(value)
	return &date
}

// --- mapping ---

func masterDataToCapability(entity *userModels.StaffMasterData) workforce.StaffMasterData {
	return workforce.StaffMasterData{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Gender: entity.Gender,
		AddressStreet: entity.AddressStreet, AddressPostalCode: entity.AddressPostalCode, AddressCity: entity.AddressCity,
		Phone: entity.Phone, Email: entity.Email, EmergencyContactName: entity.EmergencyContactName,
		EmergencyContactPhone: entity.EmergencyContactPhone, EntryDate: optionalDateString(entity.EntryDate),
		ContractEndDate: optionalDateString(entity.ContractEndDate), ProbationEndDate: optionalDateString(entity.ProbationEndDate),
		WeeklyHours: entity.WeeklyHours, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func applyMasterDataToLegacy(entity *userModels.StaffMasterData, value workforce.StaffMasterData) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Gender = value.Gender
	entity.AddressStreet = value.AddressStreet
	entity.AddressPostalCode = value.AddressPostalCode
	entity.AddressCity = value.AddressCity
	entity.Phone = value.Phone
	entity.Email = value.Email
	entity.EmergencyContactName = value.EmergencyContactName
	entity.EmergencyContactPhone = value.EmergencyContactPhone
	entity.EntryDate = optionalDate(value.EntryDate)
	entity.ContractEndDate = optionalDate(value.ContractEndDate)
	entity.ProbationEndDate = optionalDate(value.ProbationEndDate)
	entity.WeeklyHours = value.WeeklyHours
}

func applyQualificationToLegacy(entity *userModels.StaffQualification, value workforce.StaffQualification) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Name = value.Name
	entity.AcquiredOn = optionalDate(value.AcquiredOn)
	entity.ExpiresOn = optionalDate(value.ExpiresOn)
}

func financialToCapability(entity *userModels.StaffFinancialData) workforce.StaffFinancialData {
	return workforce.StaffFinancialData{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, IBAN: entity.IBAN, TaxID: entity.TaxID,
		SocialSecurityNumber: entity.SocialSecurityNumber, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func applyFinancialToLegacy(entity *userModels.StaffFinancialData, value workforce.StaffFinancialData) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.IBAN = value.IBAN
	entity.TaxID = value.TaxID
	entity.SocialSecurityNumber = value.SocialSecurityNumber
}

func documentToCapability(entity *userModels.StaffDocument) workforce.StaffDocument {
	return workforce.StaffDocument{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Category: entity.Category,
		FilenameDisplay: entity.FilenameDisplay, FilenameStored: entity.FilenameStored, SizeBytes: entity.SizeBytes,
		ContentType: entity.ContentType, UploadedBy: entity.UploadedBy, DeletedAt: entity.DeletedAt, DeletedBy: entity.DeletedBy,
		FileDeletedAt: entity.FileDeletedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func documentToLegacy(value workforce.StaffDocument) *userModels.StaffDocument {
	entity := &userModels.StaffDocument{}
	applyDocumentToLegacy(entity, value)
	return entity
}

func applyDocumentToLegacy(entity *userModels.StaffDocument, value workforce.StaffDocument) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Category = value.Category
	entity.FilenameDisplay = value.FilenameDisplay
	entity.FilenameStored = value.FilenameStored
	entity.SizeBytes = value.SizeBytes
	entity.ContentType = value.ContentType
	entity.UploadedBy = value.UploadedBy
	entity.DeletedAt = value.DeletedAt
	entity.DeletedBy = value.DeletedBy
	entity.FileDeletedAt = value.FileDeletedAt
}
