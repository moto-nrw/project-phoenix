package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// TraegerPhaseCopyStateReads read the state of a copy; the copy commands
// serve it unchanged.
type TraegerPhaseCopyStateReads interface {
	TraegerPhaseCopyState(ctx context.Context, sourceID int64, lock bool) (enrollment.TraegerCopyState, error)
}

// TraegerPhaseCopyStore reads and links the copy of a Träger phase in the
// school's tenant transaction.
type TraegerPhaseCopyStore interface {
	TraegerPhaseCopyStateReads
	LinkTraegerPhaseCopy(ctx context.Context, copyID, sourceID int64, version int, values map[string]any) error
	ReleaseTraegerPhaseCopy(ctx context.Context, copyID int64) error
}

// TraegerPhaseCopies applies a Träger phase in one school through the
// school's own phase commands (ADR 0195 sections 5 to 7). It runs in the
// school's tenant transaction, which the caller opens.
type TraegerPhaseCopies struct {
	TraegerPhaseCopyStateReads
	phases *Phases
	store  TraegerPhaseCopyStore
}

var _ enrollment.TraegerPhaseCopies = (*TraegerPhaseCopies)(nil)

// ApplyTraegerPhase creates the copy or brings it to the version. A field
// the school changed keeps the school's value; every other field of the
// Träger takes the version's value. The school's own fields stay.
func (s *TraegerPhaseCopies) ApplyTraegerPhase(ctx context.Context, copy enrollment.TraegerPhaseCopy) (enrollment.TraegerPhaseApplied, error) {
	if copy.SourceID <= 0 || copy.Version <= 0 {
		return enrollment.TraegerPhaseApplied{}, fmt.Errorf("%w: träger source and version are required", enrollment.ErrInvalidPhase)
	}
	traeger := traegerPhaseValues(copy.Fields, true)
	state, err := s.store.TraegerPhaseCopyState(ctx, copy.SourceID, true)
	if err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	if !state.Exists {
		return s.create(ctx, copy, traeger)
	}
	phase, err := s.phases.PhaseByID(ctx, state.CopyID)
	if err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	deviations := []string{}
	if phase.Traeger != nil {
		deviations = phase.Traeger.Deviations
	}
	school := traegerPhaseValues(traegerPhaseFieldsOf(phase), phase.IsActive)
	merged, changes := mergeTraegerValues(traeger, school, deviations)
	if err := applyTraegerPhaseValues(phase, merged); err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	if err := s.phases.UpdatePhase(ctx, phase); err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	if err := s.store.LinkTraegerPhaseCopy(ctx, phase.ID, copy.SourceID, copy.Version, traeger); err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	return enrollment.TraegerPhaseApplied{CopyID: phase.ID, Fields: changes}, nil
}

func (s *TraegerPhaseCopies) create(ctx context.Context, copy enrollment.TraegerPhaseCopy, traeger map[string]any) (enrollment.TraegerPhaseApplied, error) {
	created, err := s.phases.CreatePhase(ctx, traegerPhaseOf(copy.Fields))
	if err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	if err := s.store.LinkTraegerPhaseCopy(ctx, created.ID, copy.SourceID, copy.Version, traeger); err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	changes := make([]enrollment.TraegerFieldChange, 0, len(traeger))
	for _, field := range sortedKeys(traeger) {
		changes = append(changes, enrollment.TraegerFieldChange{Field: field, State: enrollment.TraegerFieldNew, Traeger: traeger[field]})
	}
	return enrollment.TraegerPhaseApplied{CopyID: created.ID, Created: true, Fields: changes}, nil
}

// WithdrawTraegerPhase removes the copy from the school: it loses its link
// and becomes inactive, and it is deleted when no enrollment request and no
// care offering hangs off it (ADR 0195 section 7).
func (s *TraegerPhaseCopies) WithdrawTraegerPhase(ctx context.Context, sourceID int64) (enrollment.TraegerWithdrawal, error) {
	state, err := s.store.TraegerPhaseCopyState(ctx, sourceID, true)
	if err != nil || !state.Exists {
		return enrollment.TraegerWithdrawal{}, err
	}
	if err := s.store.ReleaseTraegerPhaseCopy(ctx, state.CopyID); err != nil {
		return enrollment.TraegerWithdrawal{}, err
	}
	requests, err := s.phases.deps.Records.CountPhaseRequests(ctx, state.CopyID)
	if err != nil {
		return enrollment.TraegerWithdrawal{}, fmt.Errorf("withdraw träger phase copy: count requests: %w", err)
	}
	offerings, err := s.phases.deps.Offerings.CountOfferingsForPhase(ctx, state.CopyID)
	if err != nil {
		return enrollment.TraegerWithdrawal{}, fmt.Errorf("withdraw träger phase copy: count care offerings: %w", err)
	}
	if requests > 0 || offerings > 0 {
		return enrollment.TraegerWithdrawal{Found: true}, nil
	}
	if err := s.phases.DeletePhase(ctx, state.CopyID); err != nil {
		return enrollment.TraegerWithdrawal{}, err
	}
	return enrollment.TraegerWithdrawal{Found: true, Deleted: true}, nil
}

// traegerPhaseValues are the Träger's fields of a phase keyed by their
// column, the shape of the copy's traeger_values.
func traegerPhaseValues(fields enrollment.TraegerPhaseFields, active bool) map[string]any {
	fields.EnrollmentOpenAt = utcInstant(fields.EnrollmentOpenAt)
	fields.EnrollmentCloseAt = utcInstant(fields.EnrollmentCloseAt)
	encoded, _ := json.Marshal(fields)
	values := map[string]any{}
	_ = json.Unmarshal(encoded, &values)
	values["is_active"] = active
	return values
}

func traegerPhaseFieldsOf(phase *enrollment.Phase) enrollment.TraegerPhaseFields {
	return enrollment.TraegerPhaseFields{
		Name: phase.Name, Kind: phase.Kind, ServiceStartDate: phase.ServiceStartDate, ServiceEndDate: phase.ServiceEndDate,
		EnrollmentOpenAt: phase.EnrollmentOpenAt, EnrollmentCloseAt: phase.EnrollmentCloseAt, Audience: phase.Audience,
		CareOverflowMode: phase.CareOverflowMode, CareOfferingSelectionMode: phase.CareOfferingSelectionMode,
	}
}

func applyTraegerPhaseValues(phase *enrollment.Phase, values map[string]any) error {
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}
	var fields enrollment.TraegerPhaseFields
	var active struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return err
	}
	if err := json.Unmarshal(encoded, &active); err != nil {
		return err
	}
	phase.Name, phase.Kind = fields.Name, fields.Kind
	phase.ServiceStartDate, phase.ServiceEndDate = fields.ServiceStartDate, fields.ServiceEndDate
	phase.EnrollmentOpenAt, phase.EnrollmentCloseAt = fields.EnrollmentOpenAt, fields.EnrollmentCloseAt
	phase.Audience, phase.CareOverflowMode = fields.Audience, fields.CareOverflowMode
	phase.CareOfferingSelectionMode, phase.IsActive = fields.CareOfferingSelectionMode, active.IsActive
	return nil
}

// mergeTraegerValues takes the Träger's value for every field the school
// did not change and keeps the school's value where it did, and reports
// the state of each field (ADR 0195 section 6).
func mergeTraegerValues(traeger, school map[string]any, deviations []string) (map[string]any, []enrollment.TraegerFieldChange) {
	deviating := make(map[string]bool, len(deviations))
	for _, field := range deviations {
		deviating[field] = true
	}
	merged := make(map[string]any, len(traeger))
	changes := make([]enrollment.TraegerFieldChange, 0, len(traeger))
	for _, field := range sortedKeys(traeger) {
		change := enrollment.TraegerFieldChange{Field: field, Traeger: traeger[field], School: school[field]}
		same := sameJSON(traeger[field], school[field])
		switch {
		case deviating[field] && !same:
			merged[field] = school[field]
			change.State = enrollment.TraegerFieldKeeps
		case same:
			merged[field] = traeger[field]
			change.State = enrollment.TraegerFieldUnchanged
		default:
			merged[field] = traeger[field]
			change.State = enrollment.TraegerFieldChanges
		}
		changes = append(changes, change)
	}
	return merged, changes
}

func sameJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
