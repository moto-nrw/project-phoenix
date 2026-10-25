package application

import (
	"encoding/json"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields instead.
const (
	careRequestCreatedBody   = "Anfrage: Dauerhafte Betreuungszeiten ändern"
	careRequestConfirmedBody = "Anfrage bestätigt, Betreuungszeiten übernommen"
)

// Pickup pill verbs. The subject in front names what the request changes:
// the Abholzeit, the Abholart (#3945), or both.
const (
	pickupRequestCreatedVerb   = "angefragt"
	pickupRequestConfirmedVerb = "bestätigt"
	pickupRequestRejectedVerb  = "abgelehnt"
)

// careRequestRefTable names the request table a pill references.
const careRequestRefTable = "schedule.care_schedule_change_requests"

// careRequestPillType maps a request kind to the pill's request_type token.
// The German bodies are authoritative only for the staff portal; the
// localized parents portal and the decision push render from this token, so
// a one-day pickup change must not travel as a permanent care_schedule
// change.
func careRequestPillType(requestKind string) string {
	if requestKind == "pickup_change" || requestKind == carerequests.KindPickupPermission {
		return "pickup_change"
	}
	return "care_schedule"
}

// pickupPillTerms reads the requested day, time and Abholart of a
// pickup-change request. It is deliberately lenient about everything else in
// the payload: the reason may be blank under a school's reason policy, and a
// pill that cannot name the day must fall back to the plain body rather than
// fail.
func pickupPillTerms(req *carerequests.Request) (carerequests.PickupChange, bool) {
	if req == nil || req.RequestKind != "pickup_change" {
		return carerequests.PickupChange{}, false
	}
	return carerequests.PickupTerms(req.Payload)
}

// germanDateLayout renders calendar dates as dd.mm.yyyy in user-facing copy.
const germanDateLayout = "02.01.2006"

// pickupPillBody is "Abholzeit angefragt", "Abholart angefragt" or
// "Abholzeit und Abholart angefragt". An undecodable payload keeps the
// Abholzeit wording every request carried before #3945.
func pickupPillBody(verb string, req *carerequests.Request) string {
	change, ok := pickupPillTerms(req)
	switch {
	case ok && change.PickupTime != nil && change.DepartureMode != "":
		return "Abholzeit und Abholart " + verb
	case ok && change.PickupTime == nil && change.DepartureMode == "" && change.PickupPerson != nil:
		return "Abholperson " + verb
	case ok && change.PickupTime == nil:
		return "Abholart " + verb
	default:
		return "Abholzeit " + verb
	}
}

// pickupPillDetail names the requested day, time and Abholart of a pickup
// change the way the staff thread shows it ("15.09.2026, 14:30 Uhr, Fährt
// Bus"), or "" when the payload cannot be read. The pill names the pickup
// appointment; the message timestamp stays the moment of the event (#3135).
func pickupPillDetail(req *carerequests.Request) string {
	change, ok := pickupPillTerms(req)
	if !ok {
		return ""
	}
	detail := change.Date.Format(germanDateLayout)
	if change.PickupTime != nil {
		detail += ", " + change.PickupTime.Format("15:04") + " Uhr"
	}
	if change.DepartureMode != "" {
		detail += ", " + carerequests.DepartureModeLabel(change.DepartureMode)
	}
	if change.PickupPerson != nil {
		detail += ", abgeholt von " + change.PickupPerson.Name
	}
	return detail
}

// permissionPillBody is the pill of a pickup permission request (#4780):
// "Abholberechtigung angefragt: Erika Muster, Nur Mo, Mi".
func permissionPillBody(verb string, req *carerequests.Request) string {
	body := "Abholberechtigung " + verb
	permission, err := carerequests.ParsePickupPermission(req.Payload)
	if err != nil {
		return body
	}
	return body + ": " + permission.PersonName + ", " + carerequests.WeekdayListLabel(permission.Weekdays)
}

// withPickupDetail appends the requested day and time to a pickup pill body.
// Weekly-plan requests and undecodable payloads keep the body unchanged.
func withPickupDetail(body string, req *carerequests.Request) string {
	if detail := pickupPillDetail(req); detail != "" {
		return body + ": " + detail
	}
	return body
}

// pickupPillPayload is the structured twin of pickupPillDetail: the requested
// calendar day, the requested time and Abholart and, when the request
// recorded one, the pickup time that applied before. Clients that localize
// render from it; the German body stays authoritative for the staff portal.
// nil for weekly-plan requests and undecodable payloads.
func pickupPillPayload(req *carerequests.Request) map[string]any {
	if req == nil || req.RequestKind != "pickup_change" {
		return nil
	}
	terms := carerequests.StoredPickupTerms(req.Payload)
	if terms == nil {
		return nil
	}
	payload := map[string]any{"date": terms.Date.String()}
	if terms.PickupTime != "" {
		payload["pickup_time"] = terms.PickupTime
	}
	if terms.PreviousPickupTime != "" {
		payload["previous_pickup_time"] = terms.PreviousPickupTime
	}
	if terms.DepartureMode != "" {
		payload["departure_mode"] = terms.DepartureMode
	}
	if terms.PickupPersonName != "" {
		payload["pickup_person"] = terms.PickupPersonName
	}
	return payload
}

// careDecisionPill is the status and body of the pill that announces a
// decision to the submitting guardian.
func careDecisionPill(req *carerequests.Request, approve bool, reason string) (status, body string) {
	if req.RequestKind == carerequests.KindPickupPermission {
		if approve {
			return careplan.ParentMessageRequestStatusDone, permissionPillBody(pickupRequestConfirmedVerb, req)
		}
		return careplan.ParentMessageRequestStatusReject, permissionPillBody(pickupRequestRejectedVerb, req) + ". Grund: " + reason
	}
	if approve {
		if req.RequestKind == "pickup_change" {
			return careplan.ParentMessageRequestStatusDone, withPickupDetail(pickupPillBody(pickupRequestConfirmedVerb, req), req)
		}
		return careplan.ParentMessageRequestStatusDone, careRequestConfirmedBody
	}
	// The staff thread must show WHICH day was refused, not only that
	// something was (#3135). Without a decodable day the generic line stays.
	if detail := pickupPillDetail(req); req.RequestKind == "pickup_change" && detail != "" {
		return careplan.ParentMessageRequestStatusReject, pickupPillBody(pickupRequestRejectedVerb, req) + ": " + detail + ". Grund: " + reason
	}
	return careplan.ParentMessageRequestStatusReject, "Anfrage abgelehnt: " + reason
}

// correctedCarePill is the status and body of the pill that announces a
// corrected decision.
func correctedCarePill(req *carerequests.Request, approve bool) (status, body string) {
	if approve {
		return careplan.ParentMessageRequestStatusDone, "Entscheidung geändert. " + withPickupDetail(pickupPillBody(pickupRequestConfirmedVerb, req), req)
	}
	return careplan.ParentMessageRequestStatusReject, "Entscheidung geändert. " + withPickupDetail(pickupPillBody(pickupRequestRejectedVerb, req), req)
}

// careCoGuardianNoticeBody names WHAT changed and, for a pickup change, WHEN.
// A weekly plan has no single date, so it names the area alone — enough for a
// co-guardian to know to look, which is the whole purpose of the line.
func careCoGuardianNoticeBody(req *carerequests.Request) string {
	if req.RequestKind == carerequests.KindPickupPermission {
		return "Betreuungsstand geändert: Abholberechtigung"
	}
	if req.RequestKind != "pickup_change" {
		return "Betreuungsstand geändert: Betreuungszeiten"
	}
	var payload struct {
		Date string `json:"date"`
	}
	if json.Unmarshal(req.Payload, &payload) != nil {
		return "Betreuungsstand geändert: Abholzeit"
	}
	if date, err := calendar.ParseDate(payload.Date); err == nil {
		return "Betreuungsstand geändert: Abholzeit " + date.Format(germanDateLayout)
	}
	return "Betreuungsstand geändert: Abholzeit"
}

// pickupChangeReason is the trimmed reason a guardian gave for a pickup
// change, or nil when the payload holds none.
func pickupChangeReason(req *carerequests.Request) *string {
	if req == nil {
		return nil
	}
	var payload struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(req.Payload, &payload) != nil {
		return nil
	}
	trimmed := strings.TrimSpace(payload.Reason)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
