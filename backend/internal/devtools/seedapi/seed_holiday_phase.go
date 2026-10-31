package api

import (
	"fmt"
	"slices"
	"strconv"
	"time"
)

// createEnrollmentPhases opens the school-year phase and, beside it, the
// holiday phase of the Herbstferien that carries the holiday care (#5139).
func (s parentEnrollmentSeedStep) createEnrollmentPhases(rt *Runtime, auth AuthRef, schemaID int64) (int64, map[string]any, int64, error) {
	phaseID, phaseBody, err := s.createEnrollmentPhase(rt, auth, schemaID)
	if err != nil {
		return 0, nil, 0, err
	}
	holidayPhaseID, err := s.createHolidayPhase(rt, auth, schemaID)
	return phaseID, phaseBody, holidayPhaseID, err
}

// careOfferingPhase creates the holiday care in the holiday phase and every
// other offering in the school-year phase (#5139).
func careOfferingPhase(offering seedCareOffering, phaseID, holidayPhaseID int64) int64 {
	if offering.holiday {
		return holidayPhaseID
	}
	return phaseID
}

// seedSubmissionPhase sends a request that books the holiday care to the
// holiday phase, every other one to the school-year phase.
func seedSubmissionPhase(offeringKeys []string, phaseID, holidayPhaseID int64) int64 {
	for _, offering := range demoCareOfferings() {
		if offering.holiday && slices.Contains(offeringKeys, offering.key) {
			return holidayPhaseID
		}
	}
	return phaseID
}

// createHolidayPhase opens the phase of the kind Ferienbetreuung for the
// Herbstferien of the running school year (#5139). Its offering is holiday
// care and never stands behind a Regeltermin, unlike Ganztag of the
// school-year phase. It uses the school-year form.
func (s parentEnrollmentSeedStep) createHolidayPhase(rt *Runtime, auth AuthRef, schemaID int64) (int64, error) {
	now := time.Now().UTC()
	first, last := holidayCareDays(todaySeedDate())
	body := map[string]any{
		"name":                         fmt.Sprintf("Ferienbetreuung Herbstferien %d", first.Year()),
		"kind":                         "holiday",
		"service_start_date":           first.String(),
		"service_end_date":             last.String(),
		"enrollment_open_at":           now.Add(-24 * time.Hour).Format(time.RFC3339),
		"enrollment_close_at":          now.AddDate(0, 2, 0).Format(time.RFC3339),
		"show_status_reason_to_parent": true,
		"care_overflow_mode":           "waitlist",
		"care_offering_selection_mode": "at_least_one",
		"audience":                     "open",
		"is_active":                    true,
		"form_schema_id":               strconv.FormatInt(schemaID, 10),
	}
	respBody, err := rt.Client.PostWithAuth(auth, "/api/enrollment/phases", body)
	if err != nil {
		return 0, fmt.Errorf("create holiday enrollment phase: %w", err)
	}
	id, err := parseEnvelopeStringID(respBody)
	if err != nil {
		return 0, fmt.Errorf("parse holiday enrollment phase response: %w", err)
	}
	return id, nil
}
