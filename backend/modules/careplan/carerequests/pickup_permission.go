package carerequests

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"
)

// Pickup permissions asked for by a family (#4780). A guardian asks the OGS
// to let a contact person collect the child on every day or only on some
// weekdays; the OGS confirms it like a one-day pickup change. The request
// carries the simple electronic signature with a drawn signature the
// guardian gave: the stored signature's id and the exact wording signed.

// KindPickupPermission is the request kind of a pickup permission.
const KindPickupPermission = "pickup_permission"

// KindPickupPerson is the care kind of the diff line that names a person:
// the one-day collector or the weekdays of a pickup permission.
const KindPickupPerson = "pickup_person"

// ErrPickupPermissionContactGone refuses an approval whose contact person is
// no longer linked to the child.
var ErrPickupPermissionContactGone = errors.New("schedule: pickup permission contact is no longer linked")

// PickupPermissionApproval is the permission state an approved request may
// grant. The parent-portal workflow checks the current People Directory
// contact under its row locks before delegating the Care Plan write.
type PickupPermissionApproval struct {
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	RelationshipID    int64
	Weekdays          []int
}

// PickupPermissionApprovals coordinates the current-contact check with the
// Care Plan permission write inside the caller's tenant transaction. The
// same check also guards a one-day collector before its approval writes a
// dated pickup note.
type PickupPermissionApprovals interface {
	ApplyPickupPermission(context.Context, PickupPermissionApproval) error
	ValidatePickupPerson(context.Context, int64, int64) error
}

// SignedStatement is the wording a guardian signed and the signature the
// Audit Platform stored for it. The stored signature carries the SHA-256 of
// Statement, so a proof can show that the wording is unchanged.
type SignedStatement struct {
	SignatureID int64
	Statement   string
}

// PickupPerson is the contact person who collects the child on one day.
// Name is the name at the time of the request, so the history stays readable
// after the contact changes or is removed.
type PickupPerson struct {
	GuardianProfileID int64
	Name              string
}

// PickupPermissionCreateInput is one pickup permission a guardian asks for.
// Weekdays empty means every day. RelationshipID is the child's link to the
// contact that the approval writes to; the flow resolves it.
type PickupPermissionCreateInput struct {
	StudentID          int64
	GuardianAccountID  int64
	GuardianProfileID  int64
	RelationshipID     int64
	PersonName         string
	Weekdays           []int
	Reason             string
	PreviousPermission PickupPermissionState
	// PreviousLabel names the person's days before the request ("Nicht
	// abholberechtigt", "An allen Tagen", "Nur Mo, Mi"), so the review and the
	// frozen decision compare with the state the family saw.
	PreviousLabel string
	Signature     SignedStatement
}

// PickupPermissionState is the permission before a request changes it. It is
// stored with the request so a staff correction can restore exactly that
// state instead of guessing from the current relationship.
type PickupPermissionState struct {
	CanPickup bool
	Weekdays  []int
}

// PickupPermission is the stored ask of a pickup permission request.
type PickupPermission struct {
	GuardianProfileID int64
	RelationshipID    int64
	PersonName        string
	// Weekdays is nil for every day.
	Weekdays  []int
	Reason    string
	Signature *SignedStatement
	// PreviousPermission is nil on legacy rows that predate correction support.
	PreviousPermission *PickupPermissionState
}

// NormalizeWeekdays sorts and deduplicates plan weekdays (1 = Monday …
// 5 = Friday). All five days, or none, mean every day and return nil.
func NormalizeWeekdays(days []int) ([]int, error) {
	out := make([]int, 0, len(days))
	for _, day := range days {
		if day < 1 || day > 5 {
			return nil, ErrInvalidPayload
		}
		if !slices.Contains(out, day) {
			out = append(out, day)
		}
	}
	slices.Sort(out)
	if len(out) == 0 || len(out) == 5 {
		return nil, nil
	}
	return out, nil
}

// PickupPermissionPayload validates the input and builds the stored payload.
func PickupPermissionPayload(input PickupPermissionCreateInput) (json.RawMessage, error) {
	name := strings.TrimSpace(input.PersonName)
	reason := strings.TrimSpace(input.Reason)
	if input.StudentID <= 0 || input.GuardianAccountID <= 0 || input.GuardianProfileID <= 0 ||
		input.RelationshipID <= 0 || name == "" || utf8.RuneCountInString(reason) > 255 {
		return nil, ErrInvalidPayload
	}
	weekdays, err := NormalizeWeekdays(input.Weekdays)
	if err != nil {
		return nil, err
	}
	previousWeekdays, err := NormalizeWeekdays(input.PreviousPermission.Weekdays)
	if err != nil {
		return nil, err
	}
	if !validSignedStatement(input.Signature) {
		return nil, ErrInvalidPayload
	}
	payload := map[string]any{
		"guardian_profile_id": input.GuardianProfileID,
		"relationship_id":     input.RelationshipID,
		"person_name":         name,
		"weekdays":            weekdays,
		"signature":           signaturePayload(input.Signature),
	}
	if reason != "" {
		payload["reason"] = reason
	}
	if previous := strings.TrimSpace(input.PreviousLabel); previous != "" {
		payload["previous_label"] = previous
	}
	payload["previous_permission"] = map[string]any{
		"can_pickup": input.PreviousPermission.CanPickup,
		"weekdays":   previousWeekdays,
	}
	return json.Marshal(payload)
}

// ParsePickupPermission reads a stored pickup permission request.
func ParsePickupPermission(raw json.RawMessage) (PickupPermission, error) {
	var payload struct {
		GuardianProfileID  int64  `json:"guardian_profile_id"`
		RelationshipID     int64  `json:"relationship_id"`
		PersonName         string `json:"person_name"`
		Weekdays           []int  `json:"weekdays"`
		Reason             string `json:"reason"`
		PreviousPermission *struct {
			CanPickup bool  `json:"can_pickup"`
			Weekdays  []int `json:"weekdays"`
		} `json:"previous_permission"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.GuardianProfileID <= 0 || payload.RelationshipID <= 0 || payload.PersonName == "" {
		return PickupPermission{}, ErrInvalidPayload
	}
	weekdays, err := NormalizeWeekdays(payload.Weekdays)
	if err != nil {
		return PickupPermission{}, err
	}
	permission := PickupPermission{
		GuardianProfileID: payload.GuardianProfileID, RelationshipID: payload.RelationshipID,
		PersonName: payload.PersonName, Weekdays: weekdays, Reason: payload.Reason,
		Signature: StoredSignature(raw),
	}
	if payload.PreviousPermission != nil {
		previous, err := NormalizeWeekdays(payload.PreviousPermission.Weekdays)
		if err != nil {
			return PickupPermission{}, err
		}
		permission.PreviousPermission = &PickupPermissionState{
			CanPickup: payload.PreviousPermission.CanPickup,
			Weekdays:  previous,
		}
	}
	return permission, nil
}

// PickupPermissionEntries renders the ask as one diff line. old is the label
// of the permission the person held before; "" leaves that side blank.
func PickupPermissionEntries(permission PickupPermission, old string) []DiffEntry {
	return []DiffEntry{{
		Label:    "Abholberechtigung · " + permission.PersonName,
		Old:      old,
		New:      WeekdayListLabel(permission.Weekdays),
		CareKind: KindPickupPerson,
	}}
}

var weekdayShortNames = [...]string{"", "Mo", "Di", "Mi", "Do", "Fr"}

// WeekdayListLabel is the German label of a pickup permission's days:
// "An allen Tagen" for nil, else the short weekday names.
func WeekdayListLabel(weekdays []int) string {
	if len(weekdays) == 0 {
		return "An allen Tagen"
	}
	names := make([]string, 0, len(weekdays))
	for _, day := range weekdays {
		if day >= 1 && day <= 5 {
			names = append(names, weekdayShortNames[day])
		}
	}
	return "Nur " + strings.Join(names, ", ")
}

// StoredSignature reads the signature reference of any request payload; nil
// for an unsigned request.
func StoredSignature(raw json.RawMessage) *SignedStatement {
	var payload struct {
		Signature *struct {
			ID        int64  `json:"id"`
			Statement string `json:"statement"`
		} `json:"signature"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.Signature == nil {
		return nil
	}
	signed := SignedStatement{SignatureID: payload.Signature.ID, Statement: payload.Signature.Statement}
	if !validSignedStatement(signed) {
		return nil
	}
	return &signed
}

// WithSignature adds the signature reference to a canonical payload. A nil
// signature returns the payload unchanged.
func WithSignature(raw json.RawMessage, signature *SignedStatement) (json.RawMessage, error) {
	if signature == nil {
		return raw, nil
	}
	if !validSignedStatement(*signature) {
		return nil, ErrInvalidPayload
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrInvalidPayload
	}
	payload["signature"] = signaturePayload(*signature)
	return json.Marshal(payload)
}

func signaturePayload(signature SignedStatement) map[string]any {
	return map[string]any{"id": signature.SignatureID, "statement": signature.Statement}
}

func validSignedStatement(signature SignedStatement) bool {
	return signature.SignatureID > 0 && strings.TrimSpace(signature.Statement) != "" &&
		utf8.RuneCountInString(signature.Statement) <= 2000
}
