package application

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// TraegerOfferingReads read the Träger care offerings; the administration
// serves them unchanged.
type TraegerOfferingReads interface {
	ListTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID int64) ([]careplan.TraegerOffering, error)
	TraegerOffering(ctx context.Context, organizationID, id int64) (careplan.TraegerOffering, error)
}

// TraegerOfferingStore persists the Träger care offerings in the Träger
// transaction.
type TraegerOfferingStore interface {
	TraegerOfferingReads
	InsertTraegerOffering(ctx context.Context, organizationID, traegerPhaseID int64, draft careplan.TraegerOfferingDraft) (int64, error)
	UpdateTraegerOfferingDraft(ctx context.Context, organizationID, id int64, draft careplan.TraegerOfferingDraft) (bool, error)
	WithdrawTraegerOffering(ctx context.Context, organizationID, id int64) (bool, error)
	InsertTraegerOfferingVersions(ctx context.Context, organizationID int64, versions map[int64]careplan.TraegerOffering, accountID int64) error
	RecordTraegerOfferingOutcomes(ctx context.Context, organizationID, id int64, outcomes []careplan.TraegerOfferingOutcome) error
}

// TraegerOfferings keeps the Träger care offerings (ADR 0195). It opens no
// transaction: the caller runs it in the Träger transaction.
type TraegerOfferings struct {
	TraegerOfferingReads
	store TraegerOfferingStore
}

var _ careplan.TraegerOfferings = (*TraegerOfferings)(nil)

// NewTraegerOfferings builds the Träger care offerings over their store.
func NewTraegerOfferings(store TraegerOfferingStore) *TraegerOfferings {
	return &TraegerOfferings{TraegerOfferingReads: store, store: store}
}

// CreateTraegerOffering validates the draft and creates the offering in the
// Träger phase.
func (s *TraegerOfferings) CreateTraegerOffering(ctx context.Context, organizationID, traegerPhaseID int64, draft careplan.TraegerOfferingDraft) (careplan.TraegerOffering, error) {
	normalized, err := normalizeTraegerOfferingDraft(traegerPhaseID, draft)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	id, err := s.store.InsertTraegerOffering(ctx, organizationID, traegerPhaseID, normalized)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	return s.store.TraegerOffering(ctx, organizationID, id)
}

// UpdateTraegerOffering replaces the draft of an offering that is not
// withdrawn. No school notices before the next release.
func (s *TraegerOfferings) UpdateTraegerOffering(ctx context.Context, organizationID, id int64, draft careplan.TraegerOfferingDraft) (careplan.TraegerOffering, error) {
	current, err := s.store.TraegerOffering(ctx, organizationID, id)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	if current.WithdrawnAt != nil {
		return careplan.TraegerOffering{}, careplan.ErrTraegerOfferingWithdrawn
	}
	normalized, err := normalizeTraegerOfferingDraft(current.TraegerPhaseID, draft)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	if _, err := s.store.UpdateTraegerOfferingDraft(ctx, organizationID, id, normalized); err != nil {
		return careplan.TraegerOffering{}, err
	}
	return s.store.TraegerOffering(ctx, organizationID, id)
}

// WithdrawTraegerOffering marks the offering withdrawn.
func (s *TraegerOfferings) WithdrawTraegerOffering(ctx context.Context, organizationID, id int64) (careplan.TraegerOffering, error) {
	found, err := s.store.WithdrawTraegerOffering(ctx, organizationID, id)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	if !found {
		return careplan.TraegerOffering{}, careplan.ErrTraegerOfferingNotFound
	}
	return s.store.TraegerOffering(ctx, organizationID, id)
}

// ReleaseTraegerOfferings writes the next version of every offering of the
// phase whose draft differs from its last version and returns all offerings
// of the phase as released. A withdrawn offering keeps its last version.
func (s *TraegerOfferings) ReleaseTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID, accountID int64) ([]careplan.TraegerOffering, error) {
	offerings, err := s.store.ListTraegerOfferings(ctx, organizationID, traegerPhaseID)
	if err != nil {
		return nil, err
	}
	versions := map[int64]careplan.TraegerOffering{}
	for index, offering := range offerings {
		if offering.WithdrawnAt != nil || (offering.Released != nil && reflect.DeepEqual(*offering.Released, offering.Draft)) {
			continue
		}
		next := 1
		if offering.ReleasedVersion != nil {
			next = *offering.ReleasedVersion + 1
		}
		draft := offering.Draft
		offerings[index].ReleasedVersion, offerings[index].Released = &next, &draft
		versions[offering.ID] = offerings[index]
	}
	if err := s.store.InsertTraegerOfferingVersions(ctx, organizationID, versions, accountID); err != nil {
		return nil, err
	}
	return offerings, nil
}

// RecordTraegerOfferingOutcomes writes the outcome of each school's copy.
func (s *TraegerOfferings) RecordTraegerOfferingOutcomes(ctx context.Context, organizationID, id int64, outcomes []careplan.TraegerOfferingOutcome) error {
	for _, outcome := range outcomes {
		switch outcome.Outcome {
		case "applied", "rejected", "stale", "withdrawn":
		default:
			return careOfferingInvalidf("outcome %q for school %d", outcome.Outcome, outcome.SchoolID)
		}
	}
	// The offering must exist for the Träger; a missing one is reported as
	// such, not as a foreign-key failure of the outcome rows.
	if _, err := s.store.TraegerOffering(ctx, organizationID, id); err != nil {
		return err
	}
	return s.store.RecordTraegerOfferingOutcomes(ctx, organizationID, id, outcomes)
}

// normalizeTraegerOfferingDraft runs the catalog's own column rules on the
// fields, so a draft the office saves is an offering every school could
// store, and checks the price and contract template forms the copy hands
// to the Elternabrechnung and the Betreuungsvertrag.
func normalizeTraegerOfferingDraft(traegerPhaseID int64, draft careplan.TraegerOfferingDraft) (careplan.TraegerOfferingDraft, error) {
	offering := traegerOfferingOf(draft.Fields, traegerPhaseID)
	if err := validateOfferingFields(&offering); err != nil {
		return draft, err
	}
	if missing := missingPickupWeekdays(offering); len(missing) > 0 {
		return draft, fmt.Errorf("%w: %s", careplan.ErrCareOfferingPickupTimesRequired, strings.Join(missing, ", "))
	}
	draft.Fields = traegerOfferingFieldsOf(offering)
	if draft.Prices == nil {
		draft.Prices = []careplan.TraegerCarePrice{}
	}
	if err := validateTraegerPrices(draft.Prices); err != nil {
		return draft, err
	}
	if draft.ContractTemplate != nil {
		if err := validateTraegerContractTemplate(*draft.ContractTemplate); err != nil {
			return draft, err
		}
	}
	return draft, nil
}

// Bounds of the Elternabrechnung's price list (ADR 0158).
const (
	traegerMaxPriceCents = 100_000_000
	traegerMaxDays       = 7
)

func validateTraegerPrices(prices []careplan.TraegerCarePrice) error {
	weekdays := map[string]bool{}
	steps := map[int]bool{}
	for _, price := range prices {
		switch {
		case (price.Weekday == "") == (price.DaysPerWeek == nil):
			return careOfferingInvalidf("a price names either a weekday or a number of days per week")
		case price.Weekday != "" && (!validTraegerPriceDay(price.Weekday) || weekdays[price.Weekday]):
			return careOfferingInvalidf("price weekday %q is unknown or repeated", price.Weekday)
		case price.DaysPerWeek != nil && (*price.DaysPerWeek < 1 || *price.DaysPerWeek > traegerMaxDays || steps[*price.DaysPerWeek]):
			return careOfferingInvalidf("price days per week %d is out of range or repeated", *price.DaysPerWeek)
		case price.MonthlyCents < 0 || price.MonthlyCents > traegerMaxPriceCents:
			return careOfferingInvalidf("monthly price is out of range")
		case price.SiblingCents != nil && (*price.SiblingCents < 0 || *price.SiblingCents > price.MonthlyCents):
			return careOfferingInvalidf("the price for a further child lies between 0 and the monthly price")
		case price.MealExtra != prices[0].MealExtra || (price.Weekday == "") != (prices[0].Weekday == ""):
			return careOfferingInvalidf("all prices of a module share one form")
		}
		if price.Weekday != "" {
			weekdays[price.Weekday] = true
		} else {
			steps[*price.DaysPerWeek] = true
		}
	}
	return nil
}

// validateTraegerContractTemplate checks the template like Care Contract
// does, so a release does not fail at the first school.
func validateTraegerContractTemplate(template careplan.TraegerContractTemplate) error {
	problems := append(traegerTermProblems(template), traegerTextProblems(template)...)
	if len(problems) > 0 {
		return careOfferingInvalidf("contract template: %s", strings.Join(problems, "; "))
	}
	return nil
}

func traegerTermProblems(template careplan.TraegerContractTemplate) []string {
	var problems []string
	switch template.TermKind {
	case "open_ended", "school_year":
		if template.TermMonths != nil {
			problems = append(problems, "term months belong to a fixed term only")
		}
	case "fixed_months":
		if template.TermMonths == nil || *template.TermMonths < 1 || *template.TermMonths > 48 {
			problems = append(problems, "a fixed term runs 1 to 48 months")
		}
	default:
		problems = append(problems, "unknown term kind")
	}
	if template.NoticeMonths < 0 || template.NoticeMonths > 12 {
		problems = append(problems, "notice runs 0 to 12 months")
	}
	if template.NoticeTo != "month_end" && template.NoticeTo != "half_year" && template.NoticeTo != "school_year_end" {
		problems = append(problems, "unknown notice date")
	}
	return problems
}

func traegerTextProblems(template careplan.TraegerContractTemplate) []string {
	var problems []string
	if template.SpecialTermination && strings.TrimSpace(template.SpecialTerminationNote) == "" {
		problems = append(problems, "a special termination needs its note")
	}
	if template.StatuteRequired && strings.TrimSpace(template.StatuteTitle) == "" {
		problems = append(problems, "a required statute needs its title")
	}
	if link := strings.TrimSpace(template.StatuteURL); link != "" && !webAddress(link) {
		problems = append(problems, "the statute link is not a web address")
	}
	if len(template.SpecialTerminationNote) > 1000 || len(template.StatuteTitle) > 200 || len(template.StatuteURL) > 500 || len(template.ContractPartner) > 300 {
		problems = append(problems, "a text of the contract template is too long")
	}
	return problems
}

func webAddress(link string) bool {
	parsed, err := url.Parse(link)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// traegerOfferingOf is a school offering holding the Träger's fields, with
// the school's own fields empty.
func traegerOfferingOf(fields careplan.TraegerOfferingFields, phaseID int64) careplan.CareOffering {
	return careplan.CareOffering{
		PhaseID: phaseID, Name: fields.Name, Description: fields.Description, DaysOfWeekMode: fields.DaysOfWeekMode,
		AvailableDays: fields.AvailableDays, IncludesHolidayCare: fields.IncludesHolidayCare, IncludesLunch: fields.IncludesLunch,
		Capacity: fields.Capacity, PriceCents: fields.PriceCents, IsActive: true, IsRequired: fields.IsRequired,
		CountsAsCare: fields.CountsAsCare, SelectionGroup: fields.SelectionGroup, SelectionRule: fields.SelectionRule,
		SortOrder: fields.SortOrder, PickupTimes: fields.PickupTimes, AutoAddGradeLevels: []int{},
		AutoAddTriggerOfferingIDs: []int64{},
	}
}

func traegerOfferingFieldsOf(offering careplan.CareOffering) careplan.TraegerOfferingFields {
	return careplan.TraegerOfferingFields{
		Name: offering.Name, Description: offering.Description, DaysOfWeekMode: offering.DaysOfWeekMode,
		AvailableDays: offering.AvailableDays, IncludesHolidayCare: offering.IncludesHolidayCare,
		IncludesLunch: offering.IncludesLunch, Capacity: offering.Capacity, PriceCents: offering.PriceCents,
		IsRequired: offering.IsRequired, CountsAsCare: offering.CountsAsCare, SelectionGroup: offering.SelectionGroup,
		SelectionRule: offering.SelectionRule, SortOrder: offering.SortOrder, PickupTimes: offering.PickupTimes,
	}
}

func validTraegerPriceDay(day string) bool {
	_, ok := offeringDayWeekday(day)
	return ok
}
