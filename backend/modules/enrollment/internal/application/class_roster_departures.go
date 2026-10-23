package application

import (
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// phaseDeparture collects the departure answers of one approved enrollment:
// the explicit allowed modes and exclusive day plan, and the legacy bus and
// pickup answers older forms asked instead.
type phaseDeparture struct {
	explicitAllowed   *departure.AllowedDepartureModes
	explicitDeparture *departure.DepartureDays
	legacyBus         departure.BusDays
	legacyPickup      departure.PickupDays
	hasLegacy         bool
}

func classRosterDepartureByDay(req *Request, child *reportChild, schemas map[int64]*enrollment.FormSchema, student *RosterStudent, companions []departure.CompanionLink) (map[string]string, error) {
	allowed, fallback, note, ok, err := classRosterDepartureFromPhase(req, child, schemas)
	if err != nil {
		return nil, err
	}
	if ok {
		// The phase form answered the plan, but the "läuft mit" links belong to
		// the CHILD and are the current, structured answer to "mit wem" either
		// way — the roster prints both sources.
		return classRosterFormatDepartureByDay(allowed, fallback, note, companions), nil
	}
	return classRosterDepartureByDayFromStudent(student, companions), nil
}

func classRosterDepartureFromPhase(req *Request, child *reportChild, schemas map[int64]*enrollment.FormSchema) (departure.AllowedDepartureModes, departure.DepartureDays, *string, bool, error) {
	if req == nil || req.SchemaID == nil || child == nil {
		return nil, nil, nil, false, nil
	}
	schema := schemas[*req.SchemaID]
	if schema == nil {
		return nil, nil, nil, false, nil
	}
	answers := phaseDeparture{legacyBus: departure.BusDays{}, legacyPickup: departure.PickupDays{}}
	for _, field := range classRosterDepartureFields(schema) {
		raw := reportFieldValue(req, child, field)
		if raw == nil {
			continue
		}
		if err := answers.read(field, raw); err != nil {
			return nil, nil, nil, false, fmt.Errorf("%s: %w", field.Key, err)
		}
	}
	allowed, fallback, ok := answers.plan()
	return allowed, fallback, classRosterCompanionNote(child), ok, nil
}

// read folds one answered departure field into the answers; a later field of
// the same kind merges into (or, for the legacy answers, replaces) the
// earlier one.
func (p *phaseDeparture) read(field enrollment.FormField, raw any) error {
	switch field.Target {
	case enrollment.TargetStudentAllowedDepartureModes:
		modes, err := decodeAllowedDepartureModes(raw)
		if err != nil {
			return err
		}
		if p.explicitAllowed != nil {
			modes = (*p.explicitAllowed).Merge(modes)
		}
		p.explicitAllowed = &modes
	case enrollment.TargetStudentDeparture:
		days, err := decodeDepartureDays(raw)
		if err != nil {
			return err
		}
		if p.explicitDeparture != nil {
			days = (*p.explicitDeparture).Merge(days)
		}
		p.explicitDeparture = &days
	case enrollment.TargetStudentBusDays, enrollment.TargetStudentBus:
		days, err := decodeBusDays(raw)
		if err != nil {
			return err
		}
		p.legacyBus = days
		p.hasLegacy = true
	case enrollment.TargetStudentPickupStatus:
		days, err := decodePickupDays(raw)
		if err != nil {
			return err
		}
		p.legacyPickup = days
		p.hasLegacy = true
	}
	return nil
}

// plan resolves the answers in precedence order: explicit allowed modes, the
// exclusive day plan, the legacy answers. ok is false without any answer.
func (p *phaseDeparture) plan() (departure.AllowedDepartureModes, departure.DepartureDays, bool) {
	if p.explicitAllowed != nil {
		allowed := p.explicitAllowed.Normalize()
		return allowed, allowed.DepartureDays(), true
	}
	if p.explicitDeparture != nil {
		days := p.explicitDeparture.Normalize()
		return departure.AllowedDepartureModesFromDeparture(days), days, true
	}
	if p.hasLegacy {
		allowed := departure.AllowedDepartureModesFromLegacy(p.legacyBus, p.legacyPickup)
		return allowed, allowed.DepartureDays(), true
	}
	return nil, nil, false
}

func classRosterDepartureFields(schema *enrollment.FormSchema) []enrollment.FormField {
	fields := make([]enrollment.FormField, 0)
	if schema == nil {
		return fields
	}
	for _, field := range schema.Fields {
		if !field.AppliesToCh {
			continue
		}
		switch field.Target {
		case enrollment.TargetStudentAllowedDepartureModes,
			enrollment.TargetStudentDeparture,
			enrollment.TargetStudentBusDays,
			enrollment.TargetStudentBus,
			enrollment.TargetStudentPickupStatus:
			fields = append(fields, field)
		}
	}
	sort.SliceStable(fields, func(i, j int) bool {
		return fields[i].SortOrder < fields[j].SortOrder
	})
	return fields
}

func classRosterCompanionNote(child *reportChild) *string {
	if child == nil || child.CustomData == nil {
		return nil
	}
	note := strings.TrimSpace(stringValue(child.CustomData[enrollment.TargetStudentDepartureCompanionNote]))
	if note == "" {
		return nil
	}
	if runes := []rune(note); len(runes) > departure.MaxDepartureCompanionNoteLen {
		note = string(runes[:departure.MaxDepartureCompanionNoteLen])
	}
	return &note
}

func classRosterDepartureByDayFromStudent(student *RosterStudent, companions []departure.CompanionLink) map[string]string {
	if student == nil {
		return classRosterFormatDepartureByDay(nil, nil, nil, nil)
	}
	return classRosterFormatDepartureByDay(student.AllowedDepartureModes, student.DepartureDays, student.DepartureCompanionNote, companions)
}

// classRosterFormatDepartureByDay renders the per-weekday rule both class
// lists print in their weekday cells (departure.DayRules, #4749).
func classRosterFormatDepartureByDay(allowed departure.AllowedDepartureModes, fallback departure.DepartureDays, companionNote *string, companions []departure.CompanionLink) map[string]string {
	note := ""
	if companionNote != nil {
		note = *companionNote
	}
	return departure.DayRules(allowed, fallback, note, companions)
}
