package carerequests

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	ErrCareRequestForbidden         = errors.New("schedule: care request forbidden")
	ErrPickupChangeConflict         = errors.New("schedule: pickup change conflicts with a staff exception")
	ErrPickupChangeAlreadyCompleted = errors.New("schedule: pickup change cannot be approved after checkout")
	ErrPickupChangeExpired          = errors.New("schedule: pickup change date has passed")
	ErrPickupChangeImpactChanged    = errors.New("schedule: pickup change affected blocks changed")
	ErrPickupChangeImpactRequired   = errors.New("schedule: pickup change impact token is required")
)

type PickupApproval struct {
	TenantID            int64
	StudentID           int64
	Payload             json.RawMessage
	ExpectedImpactToken *string
	RequireImpactToken  bool
}

// PickupApplied names the rows an approval wrote, so a later correction can
// take back exactly those. A zero ID means that leg was not requested.
type PickupApplied struct {
	PickupExceptionID    int64
	DepartureExceptionID int64
	// PickupNoteID is the dated pickup note that names the day's collector
	// (#4780).
	PickupNoteID int64
}

// PickupApprovals applies the pickup leg of an already authorized request
// inside the caller's tenant transaction. The decision and snapshot remain
// part of that same transaction.
type PickupApprovals interface {
	ApplyPickupApproval(context.Context, PickupApproval) (PickupApplied, error)
	// SaveApprovedException writes an authorized staff value. The caller must
	// hold the student/day lock and synchronize auto-excusal in the same transaction.
	SaveApprovedException(context.Context, int64, int64, calendar.Date, time.Time, string, int64) (int64, error)
}

// PickupChange is the stored ask of one Abholänderungsanfrage: a Gehzeit, an
// Abholart, or both, for one day (#3945). A request without a time leaves the
// Gehzeit of that day untouched; one without a mode leaves the Abholart.
type PickupChange struct {
	Date          calendar.Date
	PickupTime    *time.Time
	DepartureMode string
	// PickupPerson is the contact who collects the child on that day (#4780);
	// nil leaves the day's collector to the standing pickup permissions.
	PickupPerson *PickupPerson
	Reason       string
}

// RequestableDepartureMode reports whether a guardian may ask for mode on one
// day. It is the weekly request's set: "in Begleitung" needs a companion the
// OGS links, so only the OGS sets it.
func RequestableDepartureMode(mode string) bool {
	switch mode {
	case "alone", "bus", "pickup":
		return true
	default:
		return false
	}
}

// PickupTerms reads presentation terms without requiring a reason. Schools
// may allow blank reasons, and history must remain readable. ok is false for
// an undecodable payload or one that asks for neither a time nor a mode.
func PickupTerms(raw json.RawMessage) (PickupChange, bool) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return PickupChange{}, false
	}
	return pickupTerms(payload)
}

func pickupTerms(payload map[string]any) (PickupChange, bool) {
	dateRaw, _ := payload["date"].(string)
	date, err := calendar.ParseDate(dateRaw)
	if err != nil {
		return PickupChange{}, false
	}
	change := PickupChange{Date: date}
	if pickupRaw, present := payload["pickup_time"]; present && pickupRaw != nil {
		text, _ := pickupRaw.(string)
		pickup, pickupErr := parseWallClock(text)
		if pickupErr != nil {
			return PickupChange{}, false
		}
		change.PickupTime = &pickup
	}
	if modeRaw, present := payload["departure_mode"]; present && modeRaw != nil {
		mode, _ := modeRaw.(string)
		if !RequestableDepartureMode(mode) {
			return PickupChange{}, false
		}
		change.DepartureMode = mode
	}
	if personRaw, present := payload["pickup_person"]; present && personRaw != nil {
		person, ok := storedPickupPerson(personRaw)
		if !ok {
			return PickupChange{}, false
		}
		change.PickupPerson = &person
	}
	if change.PickupTime == nil && change.DepartureMode == "" && change.PickupPerson == nil {
		return PickupChange{}, false
	}
	return change, true
}

func storedPickupPerson(raw any) (PickupPerson, bool) {
	fields, _ := raw.(map[string]any)
	id, _ := fields["guardian_profile_id"].(float64)
	name, _ := fields["name"].(string)
	if id <= 0 || strings.TrimSpace(name) == "" {
		return PickupPerson{}, false
	}
	return PickupPerson{GuardianProfileID: int64(id), Name: name}, true
}

// PickupPersonAllowed reports whether a one-day collector fits the requested
// Abholart: a child who goes alone or by bus is not collected.
func PickupPersonAllowed(departureMode string) bool {
	return departureMode == "" || departureMode == "pickup"
}

// ParsePickup is the strict reading the apply paths use: a valid day, at
// least one leg, and the stored reason.
func ParsePickup(raw json.RawMessage) (PickupChange, error) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return PickupChange{}, ErrInvalidPayload
	}
	change, ok := pickupTerms(payload)
	reason, reasonOK := payload["reason"].(string)
	reason = strings.TrimSpace(reason)
	if !ok || !reasonOK || reason == "" || utf8.RuneCountInString(reason) > 255 {
		return PickupChange{}, ErrInvalidPayload
	}
	change.Reason = reason
	return change, nil
}

// PickupImpactContent canonically encodes the exact ordered blocks displayed to
// the reviewer. The composition root supplies the SHA-256 fingerprint primitive.
func PickupImpactContent(blocks []Block) []byte {
	var content strings.Builder
	for _, block := range blocks {
		content.WriteString(strconv.FormatInt(block.ID, 10))
		content.WriteByte(0)
		content.WriteString(block.Title)
		content.WriteByte(0)
		content.WriteString(block.StartTime.Format("15:04:05"))
		content.WriteByte(0)
		content.WriteString(block.EndTime.Format("15:04:05"))
		content.WriteByte(0)
	}
	return []byte(content.String())
}
