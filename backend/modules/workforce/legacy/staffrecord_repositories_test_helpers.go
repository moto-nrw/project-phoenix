package legacy

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The adapters in this file keep the retained models/users personnel record
// contracts alive on top of the Workforce capability while their consumers
// migrate (#2690): Stammdaten, qualifications, bank and tax data. They
// perform no persistence.

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
