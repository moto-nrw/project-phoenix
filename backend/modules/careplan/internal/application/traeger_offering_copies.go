package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// Field states of a copy, the same as Enrollment's for a Träger phase.
const (
	traegerFieldNew       = "new"
	traegerFieldChanges   = "changes"
	traegerFieldKeeps     = "keeps"
	traegerFieldUnchanged = "unchanged"
)

// TraegerOfferingCopyStateReads read the state of copies; the copy commands
// serve it unchanged.
type TraegerOfferingCopyStateReads interface {
	TraegerOfferingCopyStates(ctx context.Context, sourceIDs []int64, lock bool) ([]careplan.TraegerOfferingCopyState, error)
}

// TraegerOfferingCopyStore reads and links the copies of Träger offerings in
// the school's tenant transaction.
type TraegerOfferingCopyStore interface {
	TraegerOfferingCopyStateReads
	LinkTraegerOfferingCopy(ctx context.Context, copyID, sourceID int64, version int, values map[string]any) error
	ReleaseTraegerOfferingCopy(ctx context.Context, copyID int64) error
}

// TraegerSavepoint runs a delete that the database may refuse without
// spoiling the school's transaction; ControlFailure reports a savepoint
// whose state cannot be trusted.
type TraegerSavepoint struct {
	Run            func(context.Context, func(context.Context) error) error
	ControlFailure func(error) bool
}

// TraegerOfferingCopies applies Träger offerings in one school through the
// catalog's own commands (ADR 0195 sections 5 to 7), in the school's tenant
// transaction, which the caller opens.
type TraegerOfferingCopies struct {
	TraegerOfferingCopyStateReads
	catalog   careplan.CareOfferingCatalog
	store     TraegerOfferingCopyStore
	savepoint TraegerSavepoint
}

var _ careplan.TraegerOfferingCopies = (*TraegerOfferingCopies)(nil)

// NewTraegerOfferingCopies builds the copy commands over the catalog, whose
// validation every copy write runs through.
func NewTraegerOfferingCopies(catalog careplan.CareOfferingCatalog, store TraegerOfferingCopyStore, savepoint TraegerSavepoint) (*TraegerOfferingCopies, error) {
	if catalog == nil || store == nil || savepoint.Run == nil || savepoint.ControlFailure == nil {
		return nil, errors.New("care plan träger offering copies: catalog, store and savepoint are required")
	}
	return &TraegerOfferingCopies{TraegerOfferingCopyStateReads: store, catalog: catalog, store: store, savepoint: savepoint}, nil
}

// ApplyTraegerOffering creates the copy in the school's phase copy or brings
// it to the version. A field the school changed keeps the school's value;
// every other field of the Träger takes the version's value. The school's
// own fields (timetable template, rules, triggers) stay.
func (s *TraegerOfferingCopies) ApplyTraegerOffering(ctx context.Context, copy careplan.TraegerOfferingCopy) (careplan.TraegerOfferingApplied, error) {
	if copy.SourceID <= 0 || copy.Version <= 0 || copy.PhaseID <= 0 {
		return careplan.TraegerOfferingApplied{}, careOfferingInvalidf("träger source, version and phase are required")
	}
	traeger := traegerOfferingValues(copy.Fields, true)
	states, err := s.store.TraegerOfferingCopyStates(ctx, []int64{copy.SourceID}, true)
	if err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	if len(states) == 0 {
		return s.create(ctx, copy, traeger)
	}
	current, err := s.catalog.FindOffering(ctx, states[0].CopyID)
	if err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	deviations := []string{}
	if current.Traeger != nil {
		deviations = current.Traeger.Deviations
	}
	school := traegerOfferingValues(traegerOfferingFieldsOf(current), current.IsActive)
	merged, changes := mergeTraegerOfferingValues(traeger, school, deviations)
	if err := applyTraegerOfferingValues(&current, merged); err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	if _, err := s.catalog.UpdateOffering(ctx, current); err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	if err := s.store.LinkTraegerOfferingCopy(ctx, current.ID, copy.SourceID, copy.Version, traeger); err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	return careplan.TraegerOfferingApplied{SourceID: copy.SourceID, CopyID: current.ID, Fields: changes}, nil
}

func (s *TraegerOfferingCopies) create(ctx context.Context, copy careplan.TraegerOfferingCopy, traeger map[string]any) (careplan.TraegerOfferingApplied, error) {
	created, err := s.catalog.CreateOffering(ctx, traegerOfferingOf(copy.Fields, copy.PhaseID))
	if err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	if err := s.store.LinkTraegerOfferingCopy(ctx, created.ID, copy.SourceID, copy.Version, traeger); err != nil {
		return careplan.TraegerOfferingApplied{}, err
	}
	changes := make([]careplan.TraegerFieldChange, 0, len(traeger))
	for _, field := range sortedTraegerKeys(traeger) {
		changes = append(changes, careplan.TraegerFieldChange{Field: field, State: traegerFieldNew, Traeger: traeger[field]})
	}
	return careplan.TraegerOfferingApplied{SourceID: copy.SourceID, CopyID: created.ID, Created: true, Fields: changes}, nil
}

// WithdrawTraegerOffering removes the copy from the school: it loses its
// link and becomes inactive, and it is deleted when no booking and no
// selection hangs off it (ADR 0195 section 7). The ON DELETE RESTRICT keys
// of bookings and selections decide; a refused delete rolls back to its
// savepoint and leaves the inactive offering.
func (s *TraegerOfferingCopies) WithdrawTraegerOffering(ctx context.Context, sourceID int64) (careplan.TraegerOfferingWithdrawal, error) {
	states, err := s.store.TraegerOfferingCopyStates(ctx, []int64{sourceID}, true)
	if err != nil || len(states) == 0 {
		return careplan.TraegerOfferingWithdrawal{}, err
	}
	copyID := states[0].CopyID
	if err := s.store.ReleaseTraegerOfferingCopy(ctx, copyID); err != nil {
		return careplan.TraegerOfferingWithdrawal{}, err
	}
	err = s.savepoint.Run(ctx, func(ctx context.Context) error { return s.catalog.DeleteOffering(ctx, copyID) })
	switch {
	case err == nil:
		return careplan.TraegerOfferingWithdrawal{Found: true, Deleted: true}, nil
	case s.savepoint.ControlFailure(err):
		return careplan.TraegerOfferingWithdrawal{}, err
	default:
		return careplan.TraegerOfferingWithdrawal{Found: true}, nil
	}
}

// traegerOfferingValues are the Träger's fields of an offering keyed by
// their column, the shape of the copy's traeger_values.
func traegerOfferingValues(fields careplan.TraegerOfferingFields, active bool) map[string]any {
	// The catalog stores no pickup times as NULL, so the Träger's value
	// says the same.
	if len(fields.PickupTimes) == 0 {
		fields.PickupTimes = nil
	}
	if fields.AvailableDays == nil {
		fields.AvailableDays = []string{}
	}
	encoded, _ := json.Marshal(fields)
	values := map[string]any{}
	_ = json.Unmarshal(encoded, &values)
	values["is_active"] = active
	return values
}

func applyTraegerOfferingValues(offering *careplan.CareOffering, values map[string]any) error {
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}
	var fields careplan.TraegerOfferingFields
	var active struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return err
	}
	if err := json.Unmarshal(encoded, &active); err != nil {
		return err
	}
	offering.Name, offering.Description, offering.DaysOfWeekMode = fields.Name, fields.Description, fields.DaysOfWeekMode
	offering.AvailableDays, offering.IncludesHolidayCare = fields.AvailableDays, fields.IncludesHolidayCare
	offering.IncludesLunch, offering.Capacity, offering.PriceCents = fields.IncludesLunch, fields.Capacity, fields.PriceCents
	offering.IsRequired, offering.CountsAsCare = fields.IsRequired, fields.CountsAsCare
	offering.SelectionGroup, offering.SelectionRule = fields.SelectionGroup, fields.SelectionRule
	offering.SortOrder, offering.PickupTimes, offering.IsActive = fields.SortOrder, fields.PickupTimes, active.IsActive
	return nil
}

// mergeTraegerOfferingValues takes the Träger's value for every field the
// school did not change and keeps the school's value where it did, and
// reports the state of each field (ADR 0195 section 6).
func mergeTraegerOfferingValues(traeger, school map[string]any, deviations []string) (map[string]any, []careplan.TraegerFieldChange) {
	deviating := make(map[string]bool, len(deviations))
	for _, field := range deviations {
		deviating[field] = true
	}
	merged := make(map[string]any, len(traeger))
	changes := make([]careplan.TraegerFieldChange, 0, len(traeger))
	for _, field := range sortedTraegerKeys(traeger) {
		change := careplan.TraegerFieldChange{Field: field, Traeger: traeger[field], School: school[field]}
		same := sameTraegerJSON(traeger[field], school[field])
		switch {
		case deviating[field] && !same:
			merged[field], change.State = school[field], traegerFieldKeeps
		case same:
			merged[field], change.State = traeger[field], traegerFieldUnchanged
		default:
			merged[field], change.State = traeger[field], traegerFieldChanges
		}
		changes = append(changes, change)
	}
	return merged, changes
}

func sameTraegerJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func sortedTraegerKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
