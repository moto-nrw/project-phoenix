package enrollment

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// This file holds the leaf value-formatters for the phase export: the
// German label maps, consent/status/grade/activation rendering, the
// jsonb composite-field decoders, and the generic stringifier. They are
// pure functions of their inputs (no HTTP, no DB) — export_handlers.go
// keeps the HTTP wiring and document assembly that calls into these.

var statusLabelsDE = map[string]string{
	enrollmentModels.ChildStatusSubmitted:          "Eingereicht",
	enrollmentModels.ChildStatusUnderReview:        "In Prüfung",
	enrollmentModels.ChildStatusApproved:           "Angenommen",
	enrollmentModels.ChildStatusWaitlisted:         "Warteliste",
	enrollmentModels.ChildStatusRejected:           "Abgelehnt",
	enrollmentModels.ChildStatusWithdrawn:          "Zurückgezogen",
	enrollmentModels.ChildStatusPendingRenewal:     "Verlängerung offen",
	enrollmentModels.ChildStatusAutoRenewed:        "Automatisch verlängert",
	enrollmentModels.ChildStatusPendingAdminReview: "Admin-Prüfung",
}

func statusLabelDE(status string) string {
	if label, ok := statusLabelsDE[status]; ok {
		return label
	}
	return status
}

func activationSummary(c *enrollmentModels.RequestChild) string {
	mode := "Geplant"
	if c.ActivationMode == enrollmentModels.ChildActivationImmediate {
		mode = "Sofort"
	}
	if c.ActivateOn != nil {
		return mode + " (" + timezone.Date(*c.ActivateOn).Format("02.01.2006") + ")"
	}
	return mode
}

func gradeLabel(grade *int16) string {
	if grade == nil {
		return ""
	}
	return strconv.Itoa(int(*grade)) + ". Klasse"
}

// schoolClassLabel is what the "Klasse"/"Zielklasse" export cell shows:
// the concrete class (e.g. "2a") when the parent picked one, otherwise
// the grade-level label ("N. Klasse") as a fallback (issue #1833).
func schoolClassLabel(concrete *string, grade *int16) string {
	if concrete != nil {
		if t := strings.TrimSpace(*concrete); t != "" {
			return t
		}
	}
	return gradeLabel(grade)
}

var consentLabelsDE = []struct {
	Key   string
	Label string
}{
	{enrollmentModels.ConsentKeyAGB, "AGB"},
	{enrollmentModels.ConsentKeyDataProcessing, "Datenverarbeitung"},
	{enrollmentModels.ConsentKeyEmailContact, "E-Mail-Kontakt"},
	{enrollmentModels.ConsentKeyPhoto, "Foto"},
}

func consentSummary(flags map[string]any) string {
	parts := make([]string, 0, len(consentLabelsDE))
	for _, c := range consentLabelsDE {
		parts = append(parts, c.Label+": "+consentLabel(flags, c.Key))
	}
	return strings.Join(parts, ", ")
}

func consentLabel(flags map[string]any, key string) string {
	if isTruthy(flags[key]) {
		return "Ja"
	}
	return "Nein"
}

var dayLabelsDE = map[string]string{
	"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do",
	"fri": "Fr", "sat": "Sa", "sun": "So",
}

func formatDayCodes(days []string) string {
	if len(days) == 0 {
		return ""
	}
	out := make([]string, 0, len(days))
	for _, d := range days {
		if label, ok := dayLabelsDE[strings.ToLower(strings.TrimSpace(d))]; ok {
			out = append(out, label)
		} else {
			out = append(out, d)
		}
	}
	return strings.Join(out, ", ")
}

func formatOfferings(offerings []enrollmentService.ChildOfferingRow) string {
	if len(offerings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(offerings))
	for _, o := range offerings {
		name := strings.TrimSpace(o.OfferingName)
		if name == "" {
			name = "Angebot #" + strconv.FormatInt(o.OfferingID, 10)
		}
		days := o.SelectedDays
		if len(days) == 0 {
			days = o.AvailableDays
		}
		if label := formatDayCodes(days); label != "" {
			name += " (" + label + ")"
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, "; ")
}

func formatCustomValue(field enrollmentModels.FormField, raw any) string {
	switch field.Type {
	case enrollmentModels.FormFieldBoolean:
		if isTruthy(raw) {
			return "Ja"
		}
		return "Nein"
	case enrollmentModels.FormFieldSelect:
		val := stringifyValue(raw)
		for _, opt := range field.Options {
			if opt.Value == val {
				return opt.Label
			}
		}
		return val
	case enrollmentModels.FormFieldWeekdaySchedule:
		return formatWeekdaySchedule(raw)
	case enrollmentModels.FormFieldWeekdayBoolean:
		return formatWeekdayBoolean(raw)
	case enrollmentModels.FormFieldWeekdayMode:
		return formatWeekdayMode(raw)
	case enrollmentModels.FormFieldWeekdayMultiMode:
		return formatWeekdayMultiMode(raw)
	case enrollmentModels.FormFieldPhoneList:
		return formatPhoneList(raw)
	case enrollmentModels.FormFieldContactList:
		return formatContactList(raw)
	default:
		return stringifyValue(raw)
	}
}

func formatWeekdayBoolean(raw any) string {
	var days enrollmentModels.WeekdayBoolean
	if !decodeComposite(raw, &days) {
		return stringifyValue(raw)
	}
	parts := make([]string, 0, len(weekdayOrder))
	for _, day := range weekdayOrder {
		if days[day] {
			label := dayLabelsDE[day]
			if label == "" {
				label = day
			}
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}

func formatWeekdayMultiMode(raw any) string {
	var modes enrollmentModels.WeekdayMultiMode
	if !decodeComposite(raw, &modes) {
		return stringifyValue(raw)
	}
	parts := make([]string, 0, len(weekdayOrder))
	for _, day := range weekdayOrder {
		rawModes := modes[day]
		if len(rawModes) == 0 {
			continue
		}
		modeParts := make([]string, 0, len(rawModes))
		for _, mode := range rawModes {
			modeLabel := departureModeLabelsDE[mode]
			if modeLabel == "" {
				modeLabel = mode
			}
			modeParts = append(modeParts, modeLabel)
		}
		label := dayLabelsDE[day]
		if label == "" {
			label = day
		}
		parts = append(parts, label+": "+strings.Join(modeParts, ", "))
	}
	return strings.Join(parts, "; ")
}

// departureModeLabelsDE renders a per-day departure mode for staff exports.
var departureModeLabelsDE = map[string]string{
	enrollmentModels.WeekdayModeAlone:       "geht zu Fuß",
	enrollmentModels.WeekdayModeBus:         "fährt Bus",
	enrollmentModels.WeekdayModePickup:      "wird abgeholt",
	enrollmentModels.WeekdayModeAccompanied: "geht mit anderem Kind",
}

// formatWeekdayMode renders mon..fri → alone/bus/pickup as
// "Mo: fährt Bus, Mi: wird abgeholt" in week order, skipping alone/unset days.
func formatWeekdayMode(raw any) string {
	if raw == nil {
		return ""
	}
	var modes enrollmentModels.WeekdayMode
	if !decodeComposite(raw, &modes) {
		return stringifyValue(raw)
	}
	if len(modes) == 0 {
		return "Geht immer alleine"
	}
	parts := make([]string, 0, len(weekdayOrder))
	hasAnswer := false
	for _, day := range weekdayOrder {
		mode := modes[day]
		if mode == "" {
			continue
		}
		hasAnswer = true
		if mode == enrollmentModels.WeekdayModeAlone {
			continue
		}
		label := dayLabelsDE[day]
		if label == "" {
			label = day
		}
		modeLabel := departureModeLabelsDE[mode]
		if modeLabel == "" {
			modeLabel = mode
		}
		parts = append(parts, label+": "+modeLabel)
	}
	if len(parts) == 0 && hasAnswer {
		return "Geht immer alleine"
	}
	return strings.Join(parts, ", ")
}

// decodeComposite re-decodes a jsonb value (already unmarshalled into
// map[string]any / []any when the request row was scanned) into a typed
// composite struct via a JSON round-trip — the same approach the
// decision service uses to consume these fields on approval. Reports
// false on any failure so the caller can fall back to the generic
// renderer rather than dropping the answer.
func decodeComposite(raw any, dst any) bool {
	b, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, dst) == nil
}

// weekdayOrder is the fixed Mo–Fr print order for a weekday schedule;
// a Go map range would otherwise emit days in random order.
var weekdayOrder = []string{"mon", "tue", "wed", "thu", "fri"}

// formatWeekdaySchedule renders mon..fri → HH:MM as "Mo 07:30, Di 07:30",
// in week order, skipping unset days. Empty/garbage falls back to the
// generic renderer.
func formatWeekdaySchedule(raw any) string {
	var sched enrollmentModels.WeekdaySchedule
	if !decodeComposite(raw, &sched) {
		return stringifyValue(raw)
	}
	parts := make([]string, 0, len(weekdayOrder))
	for _, day := range weekdayOrder {
		hhmm := strings.TrimSpace(sched[day])
		if hhmm == "" {
			continue
		}
		label := dayLabelsDE[day]
		if label == "" {
			label = day
		}
		parts = append(parts, label+" "+hhmm)
	}
	return strings.Join(parts, ", ")
}

var phoneTypeLabelsDE = map[string]string{
	"mobile": "Mobil",
	"home":   "Festnetz",
	"work":   "Arbeit",
	"other":  "Sonstige",
}

// formatPhoneEntries renders a list of phone entries as
// "Mobil: 0151…, Festnetz: …" — explicit label wins over the type label.
// Shared by the phone_list field and the per-contact phone numbers.
func formatPhoneEntries(entries []enrollmentModels.PhoneEntry) string {
	parts := make([]string, 0, len(entries))
	for _, p := range entries {
		num := strings.TrimSpace(p.PhoneNumber)
		if num == "" {
			continue
		}
		label := strings.TrimSpace(p.Label)
		if label == "" {
			label = phoneTypeLabelsDE[p.PhoneType]
		}
		entry := num
		if label != "" {
			entry = label + ": " + num
		}
		if p.IsPrimary {
			entry += " (primär)"
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ", ")
}

func formatPhoneList(raw any) string {
	var entries []enrollmentModels.PhoneEntry
	if !decodeComposite(raw, &entries) {
		return stringifyValue(raw)
	}
	return formatPhoneEntries(entries)
}

// formatContactList renders each additional contact / pickup-authorised
// person as "Vorname Nachname (Beziehung; Tel: …; abholberechtigt,
// Notfallkontakt)". The pickup/emergency flags are the point of the
// offline export — in a system outage this is who may collect the child.
func formatContactList(raw any) string {
	var contacts []enrollmentModels.ContactEntry
	if !decodeComposite(raw, &contacts) {
		return stringifyValue(raw)
	}
	parts := make([]string, 0, len(contacts))
	for _, c := range contacts {
		parts = append(parts, formatContactEntry(c))
	}
	return strings.Join(parts, " | ")
}

// formatContactEntry renders one contact as "Vorname Nachname (Beziehung;
// Tel: …; E-Mail: …; abholberechtigt, Notfallkontakt)".
func formatContactEntry(c enrollmentModels.ContactEntry) string {
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name == "" {
		name = "Kontakt"
	}
	extras := make([]string, 0, 4)
	if rel := strings.TrimSpace(c.RelationshipType); rel != "" {
		extras = append(extras, rel)
	}
	if phones := formatPhoneEntries(c.PhoneNumbers); phones != "" {
		extras = append(extras, "Tel: "+phones)
	}
	// ContactEntry.Validate accepts email OR phone, so an authorised
	// pickup/emergency contact may carry only an email address. Phone is
	// the priority channel in an outage, so email follows it — but it must
	// never be dropped: for an email-only contact it is the sole way to
	// reach them.
	if email := strings.TrimSpace(c.Email); email != "" {
		extras = append(extras, "E-Mail: "+email)
	}
	flags := make([]string, 0, 2)
	if c.CanPickup {
		flags = append(flags, "abholberechtigt")
	}
	if c.IsEmergencyContact {
		flags = append(flags, "Notfallkontakt")
	}
	if len(flags) > 0 {
		extras = append(extras, strings.Join(flags, ", "))
	}
	if len(extras) > 0 {
		name += " (" + strings.Join(extras, "; ") + ")"
	}
	return name
}

// stringifyValue renders an arbitrary JSON value (from a jsonb column)
// to a compact human string without dropping data.
func stringifyValue(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "Ja"
		}
		return "Nein"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, stringifyValue(item))
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		keys := slices.Sorted(maps.Keys(v))
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+stringifyValue(v[k]))
		}
		return strings.Join(parts, "; ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func isTruthy(raw any) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "ja" || s == "1" || s == "yes"
	case float64:
		return v != 0
	default:
		return false
	}
}

func timeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("02.01.2006 15:04")
}

func cloneValues(src map[listexport.ColumnID]string) map[listexport.ColumnID]string {
	dst := make(map[listexport.ColumnID]string, len(src)+12)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
