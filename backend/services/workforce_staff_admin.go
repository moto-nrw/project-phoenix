package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// The adapters in this file serve the public Workforce personnel-record
// contracts from the retained people-directory services (#2690). Lookups pass
// their failures through unchanged so the not-found classification of the
// HTTP layer keeps working; the service sentinels are reported as the public
// kinds with their wording intact.

var staffAdminSentinels = []struct {
	legacy error
	kind   error
}{
	{users.ErrStaffDocumentForbidden, workforce.ErrStaffDocumentForbidden},
	{users.ErrStaffDocumentInvalid, workforce.ErrStaffDocumentInvalid},
	{users.ErrPersonnelNumberTaken, workforce.ErrPersonnelNumberTaken},
	{users.ErrPersonnelNumberInvalid, workforce.ErrPersonnelNumberInvalid},
	{users.ErrStaffStammdatenInvalid, workforce.ErrStaffStammdatenInvalid},
}

func mapStaffAdminFailure(err error) error {
	if err == nil {
		return nil
	}
	for _, pair := range staffAdminSentinels {
		if errors.Is(err, pair.legacy) {
			return &workforce.StaffAdminError{Kind: pair.kind, Cause: err}
		}
	}
	return err
}

// parseStaffAdminDate turns an optional public YYYY-MM-DD day into the
// calendar type the retained services take.
func parseStaffAdminDate(value *string, field string) (*timezone.Date, error) {
	if value == nil {
		return nil, nil
	}
	date, err := timezone.ParseDate(*value)
	if err != nil {
		return nil, &workforce.StaffAdminError{Kind: workforce.ErrStaffStammdatenInvalid, Cause: fmt.Errorf("%s must be YYYY-MM-DD", field)}
	}
	return &date, nil
}

// --- directory --------------------------------------------------------------

type staffDirectoryCapability struct {
	people users.PersonService
}

// StaffDirectoryCapability serves workforce.StaffDirectory from the retained
// person service.
func StaffDirectoryCapability(people users.PersonService) workforce.StaffDirectory {
	if people == nil {
		panic("staff directory capability: person service is required")
	}
	return staffDirectoryCapability{people: people}
}

func publicPerson(entity *userModels.Person) *workforce.Person {
	if entity == nil {
		return nil
	}
	return &workforce.Person{
		ID: entity.ID, FirstName: entity.FirstName, LastName: entity.LastName, Birthday: publicOptionalDate(entity.Birthday), AccountID: entity.AccountID,
	}
}

func publicStaffProfile(entity *userModels.Staff) *workforce.StaffProfile {
	if entity == nil {
		return nil
	}
	profile := &workforce.StaffProfile{
		ID: entity.ID, PersonID: entity.PersonID, EmploymentType: entity.EmploymentType, WorkTimeModelID: entity.WorkTimeModelID,
		RotationAnchorDate: publicOptionalDate(entity.RotationAnchorDate), PersonnelNumber: entity.PersonnelNumber,
	}
	if entity.Person != nil {
		profile.FirstName = entity.Person.FirstName
		profile.LastName = entity.Person.LastName
		profile.Birthday = publicOptionalDate(entity.Person.Birthday)
		profile.AccountID = entity.Person.AccountID
	}
	return profile
}

func publicStaffMasterData(entity *userModels.StaffMasterData) *workforce.StaffMasterData {
	if entity == nil {
		return nil
	}
	return &workforce.StaffMasterData{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Gender: entity.Gender, AddressStreet: entity.AddressStreet,
		AddressPostalCode: entity.AddressPostalCode, AddressCity: entity.AddressCity, Phone: entity.Phone, Email: entity.Email,
		EmergencyContactName: entity.EmergencyContactName, EmergencyContactPhone: entity.EmergencyContactPhone,
		EntryDate: publicOptionalDate(entity.EntryDate), ContractEndDate: publicOptionalDate(entity.ContractEndDate),
		ProbationEndDate: publicOptionalDate(entity.ProbationEndDate), WeeklyHours: entity.WeeklyHours,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func (c staffDirectoryCapability) PersonByAccountID(ctx context.Context, accountID int64) (*workforce.Person, error) {
	person, err := c.people.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return publicPerson(person), nil
}

func (c staffDirectoryCapability) StaffByPersonID(ctx context.Context, personID int64) (*workforce.StaffProfile, error) {
	staff, err := c.people.GetStaffByPersonID(ctx, personID)
	if err != nil {
		return nil, err
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) StaffByID(ctx context.Context, staffID int64) (*workforce.StaffProfile, error) {
	staff, err := c.people.GetStaffByID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error) {
	return c.people.ResolveStaffIDByAccountID(ctx, accountID)
}

func (c staffDirectoryCapability) UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*workforce.StaffProfile, error) {
	staff, err := c.people.UpdatePersonnelNumber(ctx, staffID, value, changedByStaffID, note)
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) StaffStammdaten(ctx context.Context, staffID int64) (*workforce.StaffStammdaten, error) {
	data, err := c.people.GetStaffStammdaten(ctx, staffID)
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	if data == nil {
		return nil, nil
	}
	result := &workforce.StaffStammdaten{MasterData: publicStaffMasterData(data.MasterData)}
	if profile := publicStaffProfile(data.Staff); profile != nil {
		result.Staff = *profile
	}
	if data.Qualifications != nil {
		result.Qualifications = make([]workforce.StaffQualification, 0, len(data.Qualifications))
		for _, qualification := range data.Qualifications {
			if qualification == nil {
				continue
			}
			result.Qualifications = append(result.Qualifications, workforce.StaffQualification{
				ID: qualification.ID, TenantID: qualification.TenantID, StaffID: qualification.StaffID, Name: qualification.Name,
				AcquiredOn: publicOptionalDate(qualification.AcquiredOn), ExpiresOn: publicOptionalDate(qualification.ExpiresOn),
				CreatedAt: qualification.CreatedAt, UpdatedAt: qualification.UpdatedAt,
			})
		}
	}
	return result, nil
}

func (c staffDirectoryCapability) UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input workforce.StammdatenPersonInput, changedByStaffID int64, note string) error {
	birthday, err := parseStaffAdminDate(input.Birthday, "birthday")
	if err != nil {
		return err
	}
	return mapStaffAdminFailure(c.people.UpdateStaffStammdatenPerson(ctx, staffID, users.StammdatenPersonInput{
		FirstName: input.FirstName, LastName: input.LastName, Birthday: birthday, Gender: input.Gender,
	}, changedByStaffID, note))
}

func (c staffDirectoryCapability) UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input workforce.StammdatenKontaktInput, changedByStaffID int64, note string) error {
	return mapStaffAdminFailure(c.people.UpdateStaffStammdatenKontakt(ctx, staffID, users.StammdatenKontaktInput(input), changedByStaffID, note))
}

func (c staffDirectoryCapability) UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input workforce.StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error {
	entry, err := parseStaffAdminDate(input.EntryDate, "entry_date")
	if err != nil {
		return err
	}
	contractEnd, err := parseStaffAdminDate(input.ContractEndDate, "contract_end_date")
	if err != nil {
		return err
	}
	probationEnd, err := parseStaffAdminDate(input.ProbationEndDate, "probation_end_date")
	if err != nil {
		return err
	}
	return mapStaffAdminFailure(c.people.UpdateStaffStammdatenArbeitsvertrag(ctx, staffID, users.StammdatenArbeitsvertragInput{
		EntryDate: entry, ContractEndDate: contractEnd, ProbationEndDate: probationEnd, WeeklyHours: input.WeeklyHours, EmploymentType: input.EmploymentType,
	}, changedByStaffID, note))
}

func (c staffDirectoryCapability) ReplaceStaffQualificationList(ctx context.Context, staffID int64, inputs []workforce.StammdatenQualificationInput, changedByStaffID int64, note string) error {
	legacyInputs := make([]users.StammdatenQualificationInput, 0, len(inputs))
	for _, input := range inputs {
		acquired, err := parseStaffAdminDate(input.AcquiredOn, "acquired_on")
		if err != nil {
			return err
		}
		expires, err := parseStaffAdminDate(input.ExpiresOn, "expires_on")
		if err != nil {
			return err
		}
		legacyInputs = append(legacyInputs, users.StammdatenQualificationInput{Name: input.Name, AcquiredOn: acquired, ExpiresOn: expires})
	}
	return mapStaffAdminFailure(c.people.ReplaceStaffQualifications(ctx, staffID, legacyInputs, changedByStaffID, note))
}

func (c staffDirectoryCapability) StaffFinancialMasked(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*workforce.StaffFinancialMasked, error) {
	masked, err := c.people.GetStaffFinancialMasked(ctx, staffID, actorAccountID, actorRole)
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	if masked == nil {
		return nil, nil
	}
	return new(workforce.StaffFinancialMasked(*masked)), nil
}

func (c staffDirectoryCapability) RevealStaffFinancial(ctx context.Context, staffID, actorAccountID int64, actorRole string) (*workforce.StaffFinancialPlain, error) {
	plain, err := c.people.RevealStaffFinancial(ctx, staffID, actorAccountID, actorRole)
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	if plain == nil {
		return nil, nil
	}
	return new(workforce.StaffFinancialPlain(*plain)), nil
}

func (c staffDirectoryCapability) UpdateStaffFinancial(ctx context.Context, staffID int64, input workforce.StammdatenFinancialInput, changedByAccountID int64, note string) error {
	return mapStaffAdminFailure(c.people.UpdateStaffFinancial(ctx, staffID, users.StammdatenFinancialInput(input), changedByAccountID, note))
}

// --- documents --------------------------------------------------------------

type staffDocumentCapability struct {
	documents users.StaffDocumentService
}

// StaffDocumentCapability serves workforce.StaffDocuments from the retained
// document service.
func StaffDocumentCapability(documents users.StaffDocumentService) workforce.StaffDocuments {
	if documents == nil {
		panic("staff document capability: document service is required")
	}
	return staffDocumentCapability{documents: documents}
}

func legacyDocumentActor(actor workforce.StaffDocumentActor) users.StaffDocumentActor {
	return users.StaffDocumentActor(actor)
}

func publicStaffDocument(entity *userModels.StaffDocument) *workforce.StaffDocument {
	if entity == nil {
		return nil
	}
	return &workforce.StaffDocument{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Category: entity.Category, FilenameDisplay: entity.FilenameDisplay,
		FilenameStored: entity.FilenameStored, SizeBytes: entity.SizeBytes, ContentType: entity.ContentType, UploadedBy: entity.UploadedBy,
		DeletedAt: entity.DeletedAt, DeletedBy: entity.DeletedBy, FileDeletedAt: entity.FileDeletedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func publicStaffDocuments(entities []*userModels.StaffDocument) []workforce.StaffDocument {
	if entities == nil {
		return nil
	}
	result := make([]workforce.StaffDocument, 0, len(entities))
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		result = append(result, *publicStaffDocument(entity))
	}
	return result
}

func publicStaffDocumentInfo(info *users.StaffDocumentInfo) *workforce.StaffDocumentInfo {
	if info == nil {
		return nil
	}
	result := &workforce.StaffDocumentInfo{RetainUntil: publicOptionalDate(info.RetainUntil), ReviewDue: publicOptionalDate(info.ReviewDue)}
	if document := publicStaffDocument(info.Document); document != nil {
		result.Document = *document
	}
	return result
}

func publicCleanups(entities []*userModels.StaffDocumentFileCleanup) []workforce.StaffDocumentFileCleanup {
	if entities == nil {
		return nil
	}
	result := make([]workforce.StaffDocumentFileCleanup, 0, len(entities))
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		result = append(result, workforce.StaffDocumentFileCleanup{
			ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, FilenameStored: entity.FilenameStored,
			RetryAfter: entity.RetryAfter, CleanedAt: entity.CleanedAt, CreatedAt: entity.CreatedAt,
		})
	}
	return result
}

func (c staffDocumentCapability) document(entity *userModels.StaffDocument, err error) (*workforce.StaffDocument, error) {
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	return publicStaffDocument(entity), nil
}

func (c staffDocumentCapability) documentList(entities []*userModels.StaffDocument, err error) ([]workforce.StaffDocument, error) {
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	return publicStaffDocuments(entities), nil
}

func (c staffDocumentCapability) ListStaffDocuments(ctx context.Context, staffID int64, category string, actor workforce.StaffDocumentActor) ([]workforce.StaffDocumentInfo, []string, error) {
	infos, categories, err := c.documents.ListStaffDocuments(ctx, staffID, category, legacyDocumentActor(actor))
	if err != nil {
		return nil, nil, mapStaffAdminFailure(err)
	}
	var result []workforce.StaffDocumentInfo
	if infos != nil {
		result = make([]workforce.StaffDocumentInfo, 0, len(infos))
		for _, info := range infos {
			if public := publicStaffDocumentInfo(info); public != nil {
				result = append(result, *public)
			}
		}
	}
	return result, categories, nil
}

func (c staffDocumentCapability) CreateStaffDocument(ctx context.Context, input workforce.CreateStaffDocumentInput, actor workforce.StaffDocumentActor) (*workforce.StaffDocumentInfo, error) {
	info, err := c.documents.CreateStaffDocument(ctx, users.CreateStaffDocumentInput(input), legacyDocumentActor(actor))
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	return publicStaffDocumentInfo(info), nil
}

func (c staffDocumentCapability) ResolveStaffDocumentDownload(ctx context.Context, staffID, documentID int64, actor workforce.StaffDocumentActor) (*workforce.StaffDocument, error) {
	return c.document(c.documents.ResolveStaffDocumentDownload(ctx, staffID, documentID, legacyDocumentActor(actor)))
}

func (c staffDocumentCapability) DeleteStaffDocument(ctx context.Context, staffID, documentID int64, actor workforce.StaffDocumentActor) (*workforce.StaffDocument, error) {
	return c.document(c.documents.DeleteStaffDocument(ctx, staffID, documentID, legacyDocumentActor(actor)))
}

func (c staffDocumentCapability) ResolveStaffDocumentCleanup(ctx context.Context, staffID, documentID int64, actor workforce.StaffDocumentActor) (*workforce.StaffDocument, error) {
	return c.document(c.documents.ResolveStaffDocumentCleanup(ctx, staffID, documentID, legacyDocumentActor(actor)))
}

func (c staffDocumentCapability) ListStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64) ([]workforce.StaffDocument, error) {
	return c.documentList(c.documents.ListStaffDocumentsPendingFileCleanup(ctx, staffID))
}

func (c staffDocumentCapability) ListOffboardedStaffDocumentsPendingFileCleanup(ctx context.Context) ([]workforce.StaffDocument, error) {
	return c.documentList(c.documents.ListOffboardedStaffDocumentsPendingFileCleanup(ctx))
}

func (c staffDocumentCapability) ListDeletedStaffDocumentsPendingFileCleanups(ctx context.Context) ([]workforce.StaffDocument, error) {
	return c.documentList(c.documents.ListDeletedStaffDocumentsPendingFileCleanups(ctx))
}

func (c staffDocumentCapability) ListDeletedStaffDocumentsPendingFileCleanup(ctx context.Context, staffID int64, actor workforce.StaffDocumentActor) ([]workforce.StaffDocument, error) {
	return c.documentList(c.documents.ListDeletedStaffDocumentsPendingFileCleanup(ctx, staffID, legacyDocumentActor(actor)))
}

func (c staffDocumentCapability) MarkStaffDocumentFileDeleted(ctx context.Context, documentID int64) error {
	return mapStaffAdminFailure(c.documents.MarkStaffDocumentFileDeleted(ctx, documentID))
}

func (c staffDocumentCapability) QueueStaffDocumentFileCleanup(ctx context.Context, staffID int64, storedName string) error {
	return mapStaffAdminFailure(c.documents.QueueStaffDocumentFileCleanup(ctx, staffID, storedName))
}

func (c staffDocumentCapability) ListQueuedStaffDocumentFileCleanup(ctx context.Context, staffID int64) ([]workforce.StaffDocumentFileCleanup, error) {
	cleanups, err := c.documents.ListQueuedStaffDocumentFileCleanup(ctx, staffID)
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	return publicCleanups(cleanups), nil
}

func (c staffDocumentCapability) ListQueuedStaffDocumentFileCleanups(ctx context.Context) ([]workforce.StaffDocumentFileCleanup, error) {
	cleanups, err := c.documents.ListQueuedStaffDocumentFileCleanups(ctx)
	if err != nil {
		return nil, mapStaffAdminFailure(err)
	}
	return publicCleanups(cleanups), nil
}

func (c staffDocumentCapability) MarkQueuedStaffDocumentFileCleanupComplete(ctx context.Context, cleanupID int64) error {
	return mapStaffAdminFailure(c.documents.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanupID))
}

func (c staffDocumentCapability) MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return mapStaffAdminFailure(c.documents.MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx, storedName))
}

func (c staffDocumentCapability) ActivateQueuedStaffDocumentFileCleanup(ctx context.Context, storedName string) error {
	return mapStaffAdminFailure(c.documents.ActivateQueuedStaffDocumentFileCleanup(ctx, storedName))
}
