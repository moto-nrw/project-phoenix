package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// Engine serves the People Directory capability over the application
// services: it maps the public vocabulary onto the domain and the domain
// errors onto the contract (#4084).
type Engine struct {
	*GuardianLifecycle
	service         *Service
	students        *StudentService
	guardians       *GuardianService
	studentAudit    *StudentAuditService
	studentConsents *StudentConsentService
	studentPhotos   *StudentPhotoService
	studentNotes    *StudentNoteService
	observe         func(ports.Observation)
}

// EngineServices are the application services the engine serves the
// capability over.
type EngineServices struct {
	Service           *Service
	Students          *StudentService
	Guardians         *GuardianService
	StudentAudit      *StudentAuditService
	StudentConsents   *StudentConsentService
	StudentPhotos     *StudentPhotoService
	StudentNotes      *StudentNoteService
	GuardianLifecycle *GuardianLifecycle
	Observe           func(ports.Observation)
}

// NewEngine serves the capability over services.
func NewEngine(services EngineServices) Engine {
	return Engine{
		GuardianLifecycle: services.GuardianLifecycle,
		service:           services.Service, students: services.Students, guardians: services.Guardians,
		studentAudit: services.StudentAudit, studentConsents: services.StudentConsents,
		studentPhotos: services.StudentPhotos, studentNotes: services.StudentNotes, observe: services.Observe,
	}
}

func (e Engine) Create(ctx context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	value, err := e.service.Create(ctx, domain.CreatePerson{
		FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return toPublic(value), MapError(err)
}

func (e Engine) Update(ctx context.Context, input peopledirectory.UpdatePerson) (peopledirectory.Person, error) {
	value, err := e.service.Update(ctx, domain.UpdatePerson{
		ID: input.ID, FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return toPublic(value), MapError(err)
}

func (e Engine) Delete(ctx context.Context, id int64) error {
	return MapError(e.service.Delete(ctx, id))
}

func (e Engine) FindByID(ctx context.Context, id int64, lock string) (peopledirectory.Person, error) {
	value, err := e.service.FindByID(ctx, id, lock)
	return toPublic(value), MapError(err)
}

func (e Engine) FindByAccount(ctx context.Context, accountID int64) (peopledirectory.Person, error) {
	value, err := e.service.FindByAccount(ctx, accountID)
	return toPublic(value), MapError(err)
}

func (e Engine) FindByTag(ctx context.Context, tagID string) (peopledirectory.Person, error) {
	value, err := e.service.FindByTag(ctx, tagID)
	return toPublic(value), MapError(err)
}

func (e Engine) ListByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	return toPublicList(values), MapError(err)
}

func (e Engine) ListAcrossTenantsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListAcrossTenantsByIDs(ctx, ids)
	return toPublicList(values), MapError(err)
}

func (e Engine) ListByTenantIDs(ctx context.Context, tenantIDs []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByTenantIDs(ctx, tenantIDs)
	return toPublicList(values), MapError(err)
}

func (e Engine) ListByAccounts(ctx context.Context, accountIDs []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByAccounts(ctx, accountIDs)
	return toPublicList(values), MapError(err)
}

func (e Engine) Search(ctx context.Context, filter peopledirectory.PersonFilter) ([]peopledirectory.Person, error) {
	values, err := e.service.Search(ctx, domain.Filter{
		FirstNamePrefix: filter.FirstNamePrefix, LastNamePrefix: filter.LastNamePrefix,
		FirstNameEquals: filter.FirstNameEquals, LastNameEquals: filter.LastNameEquals,
		FullNameContains: filter.FullNameContains, TagID: filter.TagID, AccountIDs: filter.AccountIDs,
		Page: filter.Page, PageSize: filter.PageSize,
	})
	return toPublicList(values), MapError(err)
}

func (e Engine) CountByTenant(ctx context.Context) (map[int64]int, error) {
	value, err := e.service.CountByTenant(ctx)
	return value, MapError(err)
}

func (e Engine) LinkAccount(ctx context.Context, personID, accountID int64) error {
	return MapError(e.service.LinkAccount(ctx, personID, accountID))
}

func (e Engine) UnlinkAccount(ctx context.Context, personID int64) error {
	return MapError(e.service.UnlinkAccount(ctx, personID))
}

func (e Engine) LinkTag(ctx context.Context, personID int64, tagID string) error {
	return MapError(e.service.LinkTag(ctx, personID, tagID))
}

func (e Engine) UnlinkTag(ctx context.Context, personID int64) error {
	return MapError(e.service.UnlinkTag(ctx, personID))
}

func (e Engine) ReleaseTags(ctx context.Context, personIDs []int64) ([]peopledirectory.ReleasedTag, error) {
	values, err := e.service.ReleaseTags(ctx, personIDs)
	result := make([]peopledirectory.ReleasedTag, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.ReleasedTag{PersonID: value.PersonID, TagID: value.TagID})
	}
	return result, MapError(err)
}

func (e Engine) RestoreTag(ctx context.Context, personID int64, tagID string) (bool, error) {
	restored, err := e.service.RestoreTag(ctx, personID, tagID)
	return restored, MapError(err)
}

func toPublic(value domain.Person) peopledirectory.Person {
	return peopledirectory.Person{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
		FirstName: value.FirstName, LastName: value.LastName, Birthday: value.Birthday,
		TagID: value.TagID, AccountID: value.AccountID, DeletedAt: value.DeletedAt,
	}
}

func toPublicList(values []domain.Person) []peopledirectory.Person {
	result := make([]peopledirectory.Person, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result
}

// MapError maps the owner errors onto the People Directory contract.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return peopledirectory.ErrPersonNotFound
	case errors.Is(err, domain.ErrTagConflict):
		return peopledirectory.ErrTagConflict
	case errors.Is(err, domain.ErrAccountConflict):
		return peopledirectory.ErrAccountConflict
	case errors.Is(err, domain.ErrStudentNotFound):
		return peopledirectory.ErrStudentNotFound
	case errors.Is(err, domain.ErrStudentLockBusy):
		return fmt.Errorf("%w: %w", peopledirectory.ErrStudentLockBusy, err)
	case errors.Is(err, domain.ErrCompanionWouldLoseDeparture):
		return peopledirectory.ErrCompanionWouldLoseDeparture
	case errors.Is(err, domain.ErrCompanionLockBusy):
		return peopledirectory.ErrCompanionLockBusy
	case errors.Is(err, domain.ErrStudentInvalid):
		// The reason is the message a handler renders, so it is kept.
		return &peopledirectory.InvalidStudentError{Reason: err.Error()}
	case errors.Is(err, domain.ErrFamilyProtectionUnchanged):
		return peopledirectory.ErrFamilyProtectionUnchanged
	case errors.Is(err, domain.ErrFamilyProtectionInvalid):
		return peopledirectory.ErrFamilyProtectionInvalid
	case errors.Is(err, domain.ErrStudentNoteNotFound):
		return peopledirectory.ErrStudentNoteNotFound
	case errors.Is(err, domain.ErrStudentNoteNotAuthor):
		return peopledirectory.ErrStudentNoteNotAuthor
	case errors.Is(err, domain.ErrStudentNoteImmutable):
		return peopledirectory.ErrStudentNoteImmutable
	case errors.Is(err, domain.ErrStudentNoteDeleteForbidden):
		return peopledirectory.ErrStudentNoteDeleteForbidden
	case errors.Is(err, domain.ErrStudentNoteInvalid):
		return peopledirectory.ErrStudentNoteInvalid
	default:
		return err
	}
}
