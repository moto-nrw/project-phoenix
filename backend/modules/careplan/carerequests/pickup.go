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

// PickupApprovals applies the pickup leg of an already authorized request
// inside the caller's tenant transaction. The decision and snapshot remain
// part of that same transaction.
type PickupApprovals interface {
	ApplyPickupApproval(context.Context, PickupApproval) (int64, error)
	// SaveApprovedException writes an authorized staff value. The caller must
	// hold the student/day lock and synchronize auto-excusal in the same transaction.
	SaveApprovedException(context.Context, int64, int64, calendar.Date, time.Time, string, int64) (int64, error)
}

// PickupTerms reads presentation terms without requiring a reason. Schools
// may allow blank reasons, and history must remain readable.
func PickupTerms(raw json.RawMessage) (calendar.Date, time.Time, bool) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return "", time.Time{}, false
	}
	return pickupTerms(payload)
}

func pickupTerms(payload map[string]any) (calendar.Date, time.Time, bool) {
	dateRaw, _ := payload["date"].(string)
	pickupRaw, _ := payload["pickup_time"].(string)
	date, dateErr := calendar.ParseDate(dateRaw)
	pickup, pickupErr := parseWallClock(pickupRaw)
	if dateErr != nil || pickupErr != nil {
		return "", time.Time{}, false
	}
	return date, pickup, true
}

func ParsePickup(raw json.RawMessage) (calendar.Date, time.Time, string, error) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return "", time.Time{}, "", ErrInvalidPayload
	}
	dateRaw, dateOK := payload["date"].(string)
	pickupRaw, pickupOK := payload["pickup_time"].(string)
	reason, reasonOK := payload["reason"].(string)
	date, dateErr := calendar.ParseDate(dateRaw)
	pickup, pickupErr := parseWallClock(pickupRaw)
	reason = strings.TrimSpace(reason)
	if !dateOK || !pickupOK || !reasonOK || dateErr != nil || pickupErr != nil || reason == "" || utf8.RuneCountInString(reason) > 255 {
		return "", time.Time{}, "", ErrInvalidPayload
	}
	return date, pickup, reason, nil
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
