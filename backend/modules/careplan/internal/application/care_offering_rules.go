package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The catalog rules below were the enrollment model's Validate until the
// catalog moved to Care Plan (#3559). Their texts are what administrators
// read, so they stay byte-identical.

const (
	daysOfWeekModeFixed        = "fixed"
	daysOfWeekModeParentChoice = "parent_choice"

	selectionRuleOptional   = "optional"
	selectionRuleExactlyOne = "exactly_one"
	selectionRuleAtLeastOne = "at_least_one"
	selectionRuleAtMostOne  = "at_most_one"

	availabilityMatchAll         = "all"
	availabilityMatchAny         = "any"
	availabilitySourceGradeLevel = "grade_level"
	availabilityOperatorIn       = "in"
	availabilityOperatorNotIn    = "not_in"

	minGradeLevel = 1
	maxGradeLevel = 13
)

var validSelectionRules = map[string]bool{
	selectionRuleOptional:   true,
	selectionRuleExactlyOne: true,
	selectionRuleAtLeastOne: true,
	selectionRuleAtMostOne:  true,
}

// offeringDayISOWeekday maps each canonical day to its ISO weekday.
var offeringDayISOWeekday = map[string]int{
	"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6, "sun": 7,
}

// offeringDayWeekday translates a stored day abbreviation ("mon") into its
// ISO weekday number (1=Monday .. 7=Sunday).
func offeringDayWeekday(day string) (int, bool) {
	weekday, ok := offeringDayISOWeekday[strings.ToLower(strings.TrimSpace(day))]
	return weekday, ok
}

type availabilityRule struct {
	Match      string                  `json:"match"`
	Conditions []availabilityCondition `json:"conditions"`
}

type availabilityCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    []int  `json:"value"`
}

// decodeAvailabilityRule reads a stored rule; nil means available to every
// child.
func decodeAvailabilityRule(raw json.RawMessage) (*availabilityRule, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	rule := new(availabilityRule)
	if err := json.Unmarshal(raw, rule); err != nil {
		return nil, fmt.Errorf("decode care offering availability rule: %w", err)
	}
	return rule, nil
}

func encodeAvailabilityRule(rule *availabilityRule) (json.RawMessage, error) {
	if rule == nil {
		return nil, nil
	}
	return json.Marshal(rule)
}

// normalizeAndValidate validates the storage-level rule contract and
// canonicalizes grade lists without changing condition order.
func (r *availabilityRule) normalizeAndValidate() error {
	if r == nil || len(r.Conditions) == 0 {
		return nil
	}
	if r.Match != availabilityMatchAll && r.Match != availabilityMatchAny {
		return fmt.Errorf("availability_rule.match must be %q or %q", availabilityMatchAll, availabilityMatchAny)
	}
	for i := range r.Conditions {
		if err := r.Conditions[i].normalizeAndValidate(i + 1); err != nil {
			return err
		}
	}
	return nil
}

func (c *availabilityCondition) normalizeAndValidate(position int) error {
	if c.Source != availabilitySourceGradeLevel {
		return fmt.Errorf("availability_rule condition %d has unknown source %q", position, c.Source)
	}
	if c.Operator != availabilityOperatorIn && c.Operator != availabilityOperatorNotIn {
		return fmt.Errorf("availability_rule condition %d has unknown operator %q", position, c.Operator)
	}
	if len(c.Value) == 0 {
		return fmt.Errorf("availability_rule condition %d requires at least one value", position)
	}
	seen := make(map[int]struct{}, len(c.Value))
	values := make([]int, 0, len(c.Value))
	for _, grade := range c.Value {
		if grade < minGradeLevel || grade > maxGradeLevel {
			return fmt.Errorf("availability_rule condition %d contains invalid grade %d", position, grade)
		}
		if _, ok := seen[grade]; ok {
			continue
		}
		seen[grade] = struct{}{}
		values = append(values, grade)
	}
	slices.Sort(values)
	c.Value = values
	return nil
}

// normalizeLoadedAvailabilityRule validates a persisted rule and returns its
// canonical form, so a corrupt row never reaches a reader.
func normalizeLoadedAvailabilityRule(offering *careplan.CareOffering) error {
	rule, err := decodeAvailabilityRule(offering.AvailabilityRule)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}
	if err := rule.normalizeAndValidate(); err != nil {
		return fmt.Errorf("care offering %d has an invalid persisted availability rule: %w", offering.ID, err)
	}
	offering.AvailabilityRule, err = encodeAvailabilityRule(rule)
	return err
}

// validateOfferingFields enforces the column-level rules of an offering and
// canonicalizes it in place: trimmed name and group, default mode and rule,
// deduplicated grades, canonical availability rule and pickup times.
func validateOfferingFields(offering *careplan.CareOffering) error {
	if err := validateOfferingIdentity(offering); err != nil {
		return err
	}
	if err := validateOfferingDays(offering); err != nil {
		return err
	}
	if offering.Capacity != nil && *offering.Capacity < 0 {
		return errors.New("capacity must be non-negative")
	}
	if offering.PriceCents != nil && *offering.PriceCents < 0 {
		return errors.New("price_cents must be non-negative")
	}
	levels, err := normalizeGradeLevelList("auto_add_grade_levels", offering.AutoAddGradeLevels)
	if err != nil {
		return err
	}
	offering.AutoAddGradeLevels = levels
	if err := normalizeOfferingAvailabilityRule(offering); err != nil {
		return err
	}
	if err := validateOfferingSelection(offering); err != nil {
		return err
	}
	times, err := normalizePickupTimes(offering.PickupTimes, offering.AvailableDays)
	if err != nil {
		return err
	}
	offering.PickupTimes = times
	// A required offering must be available to every child, so it cannot
	// carry a hard capacity limit - a full offering would block every new
	// enrollment in the phase.
	if offering.IsRequired && offering.Capacity != nil {
		return errors.New("a required care offering must not have a capacity limit")
	}
	return nil
}

func validateOfferingIdentity(offering *careplan.CareOffering) error {
	offering.Name = strings.TrimSpace(offering.Name)
	if offering.Name == "" {
		return errors.New("care offering name is required")
	}
	if offering.PhaseID == 0 {
		return errors.New("phase_id is required")
	}
	return nil
}

func validateOfferingDays(offering *careplan.CareOffering) error {
	if offering.DaysOfWeekMode == "" {
		offering.DaysOfWeekMode = daysOfWeekModeFixed
	}
	if offering.DaysOfWeekMode != daysOfWeekModeFixed && offering.DaysOfWeekMode != daysOfWeekModeParentChoice {
		return fmt.Errorf("days_of_week_mode must be 'fixed' or 'parent_choice', got %q", offering.DaysOfWeekMode)
	}
	// The weekday selection is a deliberate input: an offering silently saved
	// with all (or no) days caused wrong enrollments in production (#1885).
	if len(offering.AvailableDays) == 0 {
		return careplan.ErrCareOfferingDaysRequired
	}
	for _, day := range offering.AvailableDays {
		if _, ok := offeringDayISOWeekday[strings.ToLower(day)]; !ok {
			return fmt.Errorf("available_days entry %q is not a known day abbreviation", day)
		}
	}
	return nil
}

func normalizeOfferingAvailabilityRule(offering *careplan.CareOffering) error {
	rule, err := decodeAvailabilityRule(offering.AvailabilityRule)
	if err != nil {
		return err
	}
	if rule == nil {
		offering.AvailabilityRule = nil
		return nil
	}
	if err := rule.normalizeAndValidate(); err != nil {
		return err
	}
	if len(rule.Conditions) == 0 {
		offering.AvailabilityRule = nil
		return nil
	}
	offering.AvailabilityRule, err = encodeAvailabilityRule(rule)
	return err
}

func validateOfferingSelection(offering *careplan.CareOffering) error {
	offering.SelectionGroup = strings.TrimSpace(offering.SelectionGroup)
	if offering.SelectionRule == "" {
		offering.SelectionRule = selectionRuleOptional
	}
	if !validSelectionRules[offering.SelectionRule] {
		return fmt.Errorf("selection_rule %q is invalid", offering.SelectionRule)
	}
	// A non-optional rule only makes sense within a named group — it
	// constrains the count across the group's members.
	if offering.SelectionRule != selectionRuleOptional && offering.SelectionGroup == "" {
		return errors.New("a selection rule requires a selection_group name")
	}
	return nil
}

// normalizePickupTimes canonicalizes the Angebots-Gehzeit map: keys are
// lowercased day codes, values zero-padded HH:MM strings (the pickup
// projection's latest-wins rule compares them lexicographically). Empty
// values are dropped; an empty result becomes nil so the column stores NULL.
func normalizePickupTimes(times map[string]string, availableDays []string) (map[string]string, error) {
	if len(times) == 0 {
		return nil, nil
	}
	available := make(map[string]bool, len(availableDays))
	for _, day := range availableDays {
		available[strings.ToLower(day)] = true
	}
	out := make(map[string]string, len(times))
	for day, hhmm := range times {
		key := strings.ToLower(strings.TrimSpace(day))
		value := strings.TrimSpace(hhmm)
		if value == "" {
			continue
		}
		if err := validatePickupDay(day, key, available); err != nil {
			return nil, err
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return nil, fmt.Errorf("pickup_times value for %q must be HH:MM, got %q", key, hhmm)
		}
		out[key] = parsed.Format("15:04")
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func validatePickupDay(day, key string, available map[string]bool) error {
	if _, ok := offeringDayISOWeekday[key]; !ok {
		return fmt.Errorf("pickup_times key %q is not a known day abbreviation", day)
	}
	if key == "sat" || key == "sun" {
		return fmt.Errorf("pickup_times day %q must be Monday through Friday", key)
	}
	if !available[key] {
		return fmt.Errorf("pickup_times day %q is not in available_days", key)
	}
	return nil
}

// normalizeGradeLevelList validates a grade-level list against the supported
// school grades, drops duplicates and keeps the entered order. An empty list
// becomes a non-nil empty slice so the jsonb column stores '[]'.
func normalizeGradeLevelList(field string, levels []int) ([]int, error) {
	if len(levels) == 0 {
		return []int{}, nil
	}
	seen := make(map[int]bool, len(levels))
	out := make([]int, 0, len(levels))
	for _, level := range levels {
		if level < minGradeLevel || level > maxGradeLevel {
			return nil, fmt.Errorf("%s contains invalid grade %d", field, level)
		}
		if seen[level] {
			continue
		}
		seen[level] = true
		out = append(out, level)
	}
	return out, nil
}

// missingPickupWeekdays lists the Monday-to-Friday days of an active,
// care-counting offering that carry no pickup time.
func missingPickupWeekdays(offering careplan.CareOffering) []string {
	if !offering.IsActive || !offering.CountsAsCare {
		return nil
	}
	missing := make([]string, 0, len(offering.AvailableDays))
	for _, day := range offering.AvailableDays {
		key := strings.ToLower(strings.TrimSpace(day))
		weekday, ok := offeringDayWeekday(key)
		if ok && weekday <= isoFriday && offering.PickupTimes[key] == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

// normalizeSelectionRule maps an empty rule to the "optional" default so a
// group's members can be compared on equal footing.
func normalizeSelectionRule(rule string) string {
	if rule == "" {
		return selectionRuleOptional
	}
	return rule
}

func normalizeTriggerOfferingIDs(targetID int64, ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || id == targetID || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func autoAddViolatesExclusiveGroup(target, trigger careplan.CareOffering) bool {
	group := strings.TrimSpace(target.SelectionGroup)
	if group == "" || strings.TrimSpace(trigger.SelectionGroup) != group {
		return false
	}
	switch normalizeSelectionRule(target.SelectionRule) {
	case selectionRuleExactlyOne, selectionRuleAtMostOne:
		return true
	default:
		return false
	}
}

// validateCatalogGroupRuleConsistency checks a whole clone set at once: every
// selection group keeps one rule.
func validateCatalogGroupRuleConsistency(offerings []careplan.CareOffering) error {
	rules := make(map[string]string)
	for _, offering := range offerings {
		group := strings.TrimSpace(offering.SelectionGroup)
		if group == "" {
			continue
		}
		rule := normalizeSelectionRule(offering.SelectionRule)
		if existing, ok := rules[group]; ok && existing != rule {
			return fmt.Errorf("%w: group %q uses both %q and %q", careplan.ErrCareOfferingGroupRuleConflict, group, existing, rule)
		}
		rules[group] = rule
	}
	return nil
}

func careOfferingInvalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", careplan.ErrCareOfferingConfigInvalid, fmt.Sprintf(format, args...))
}

func wrapCareOfferingInvalid(err error, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %w", careplan.ErrCareOfferingConfigInvalid, fmt.Sprintf(format, args...), err)
}
