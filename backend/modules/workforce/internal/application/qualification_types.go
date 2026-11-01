package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// Nachweisarten (#4808): the kinds of proof of a school.

// QualificationTypes administers the kinds over the record rows.
type QualificationTypes struct{ records qualificationRecords }

// NewQualificationTypes builds the administration of the kinds.
func NewQualificationTypes(store ports.Store, clock ports.Clock) *QualificationTypes {
	return &QualificationTypes{records: newQualificationRecords(store, clock)}
}

// ListQualificationTypes returns the school's kinds and adds the standard
// kinds it does not have yet, so a new school starts with the standard list.
func (q *QualificationTypes) ListQualificationTypes(ctx context.Context) ([]workforce.QualificationType, error) {
	if err := q.records.ensureStandardTypes(ctx); err != nil {
		return nil, mapQualificationTypeError(err)
	}
	result, _, err := q.records.store.ListQualificationTypes(ctx)
	if err != nil {
		return nil, mapQualificationTypeError(err)
	}
	return qualificationTypesToPublic(result), nil
}

// CreateQualificationType adds a kind of the school after the existing ones.
func (q *QualificationTypes) CreateQualificationType(ctx context.Context, input workforce.QualificationTypeInput) (workforce.QualificationType, error) {
	value, err := qualificationTypeFromInput(input).Normalize()
	if err == nil && input.Archived {
		err = invalidStaffRecord("a new qualification type cannot be archived")
	}
	if err != nil {
		return workforce.QualificationType{}, mapQualificationTypeError(err)
	}
	if err := q.records.ensureStandardTypes(ctx); err != nil {
		return workforce.QualificationType{}, mapQualificationTypeError(err)
	}
	existing, _, err := q.records.store.ListQualificationTypes(ctx)
	if err != nil {
		return workforce.QualificationType{}, mapQualificationTypeError(err)
	}
	for _, kind := range existing {
		value.SortOrder = max(value.SortOrder, kind.SortOrder+1)
	}
	created, _, err := q.records.store.InsertQualificationType(ctx, value)
	if err != nil {
		return workforce.QualificationType{}, mapQualificationTypeError(err)
	}
	return qualificationTypeToPublic(created), nil
}

// UpdateQualificationType rewrites a kind. Archiving keeps the first archive
// time; restoring a kind whose name a live kind took meanwhile is refused as
// a taken name.
func (q *QualificationTypes) UpdateQualificationType(ctx context.Context, id int64, input workforce.QualificationTypeInput) (workforce.QualificationType, error) {
	updated, err := q.updateQualificationType(ctx, id, input)
	if err != nil {
		return workforce.QualificationType{}, mapQualificationTypeError(err)
	}
	return qualificationTypeToPublic(updated), nil
}

func (q *QualificationTypes) updateQualificationType(ctx context.Context, id int64, input workforce.QualificationTypeInput) (domain.QualificationType, error) {
	if id <= 0 {
		return domain.QualificationType{}, domain.ErrQualificationTypeNotFound
	}
	value, err := qualificationTypeFromInput(input).Normalize()
	if err != nil {
		return domain.QualificationType{}, err
	}
	existing, _, err := q.records.store.ListQualificationTypes(ctx)
	if err != nil {
		return domain.QualificationType{}, err
	}
	current, ok := findQualificationType(existing, id)
	if !ok {
		return domain.QualificationType{}, domain.ErrQualificationTypeNotFound
	}
	value.ID = id
	if input.Archived {
		value.ArchivedAt = current.ArchivedAt
		if value.ArchivedAt == nil {
			value.ArchivedAt = new(q.records.clock.Now())
		}
	}
	updated, found, _, err := q.records.store.UpdateQualificationType(ctx, value)
	if err == nil && !found {
		err = domain.ErrQualificationTypeNotFound
	}
	return updated, err
}

func findQualificationType(types []domain.QualificationType, id int64) (domain.QualificationType, bool) {
	for _, kind := range types {
		if kind.ID == id {
			return kind, true
		}
	}
	return domain.QualificationType{}, false
}

func qualificationTypeFromInput(input workforce.QualificationTypeInput) domain.QualificationType {
	return domain.QualificationType{
		Name: input.Name, ValidityMonths: input.ValidityMonths,
		ReminderLeadWeeks: input.ReminderLeadWeeks, Mandatory: input.Mandatory,
	}
}

func qualificationTypeToPublic(value domain.QualificationType) workforce.QualificationType {
	return workforce.QualificationType{
		ID: value.ID, StandardKey: value.StandardKey, Name: value.Name, ValidityMonths: value.ValidityMonths,
		ReminderLeadWeeks: value.ReminderLeadWeeks, Mandatory: value.Mandatory, Archived: value.Archived(),
	}
}

func qualificationTypesToPublic(values []domain.QualificationType) []workforce.QualificationType {
	result := make([]workforce.QualificationType, 0, len(values))
	for _, value := range values {
		result = append(result, qualificationTypeToPublic(value))
	}
	return result
}

func mapQualificationTypeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrQualificationTypeNameTaken):
		return workforce.ErrQualificationTypeNameTaken
	case errors.Is(err, domain.ErrQualificationTypeNotFound):
		return workforce.ErrQualificationTypeNotFound
	default:
		return mapStaffRecordError(err)
	}
}
