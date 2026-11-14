package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The files of the holiday registrations (#6691): one read for one
// registration or a whole period, with a statement count that does not grow
// with the registrations, recorded in the audit trail before it answers.

// holidayRecordFormat names the file in the audit trail, beside the formats
// of the list export.
const holidayRecordFormat = "pdf"

// HolidayRecordStore reads what a file adds to the review's reads, for a
// whole period or the one registration registrationID names.
type HolidayRecordStore interface {
	// ReadHolidayRecordCriteria includes active criteria and any historical
	// criteria answered by the selected registrations, including archived ones.
	ReadHolidayRecordCriteria(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayCriterion, error)
	ReadHolidayRecordContacts(ctx context.Context, organizationID, periodID, registrationID int64) (map[int64][]enrollment.HolidayRegistrationContact, error)
	ReadHolidayRecordHealth(ctx context.Context, organizationID, periodID, registrationID int64) (map[int64]enrollment.HolidaySealedRegistrationHealth, error)
	// AppendHolidayHealthReads records a health read of every registration
	// named, with details as a JSON object.
	AppendHolidayHealthReads(ctx context.Context, organizationID, periodID int64, registrationIDs []int64, details string, actorAccountID int64) error
}

// HolidayRegistrationRecords serves enrollment.HolidayRegistrationRecords
// over the review's reads; NewHolidayRegistrationReview builds it.
type HolidayRegistrationRecords struct {
	review *HolidayRegistrationReview
	store  HolidayRecordStore
}

var _ enrollment.HolidayRegistrationRecords = (*HolidayRegistrationRecords)(nil)

// HolidayRegistrationRecord reads one registration of the period and
// records the read of it, and of its health information when asked.
func (r *HolidayRegistrationRecords) HolidayRegistrationRecord(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, withHealth bool) (enrollment.HolidayRegistrationRecordSet, error) {
	if err := holidayReviewKey(reviewer, periodID, registrationID); err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	set, err := r.read(ctx, reviewer, periodID, registrationID, enrollment.HolidayRegistrationFilter{}, withHealth)
	if err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	if len(set.Records) == 0 {
		return enrollment.HolidayRegistrationRecordSet{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	if err := r.review.audit(ctx, reviewer, periodID, registrationID, enrollment.HolidayAuditRead,
		map[string]any{"format": holidayRecordFormat}); err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	return set, r.recordHealthReads(ctx, reviewer, periodID, set, withHealth)
}

// HolidayRegistrationRecords reads the registrations of the filter and
// records the export, and with health information a health read of each.
func (r *HolidayRegistrationRecords) HolidayRegistrationRecords(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter, withHealth bool) (enrollment.HolidayRegistrationRecordSet, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	if err := validateHolidayRegistrationFilter(filter); err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	set, err := r.read(ctx, reviewer, periodID, 0, filter, withHealth)
	if err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	if err := r.review.audit(ctx, reviewer, periodID, 0, enrollment.HolidayAuditExport, map[string]any{
		"filter": holidayFilterDetails(filter), "format": holidayRecordFormat, "rows": len(set.Records), "health": withHealth,
	}); err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	return set, r.recordHealthReads(ctx, reviewer, periodID, set, withHealth)
}

func (r *HolidayRegistrationRecords) recordHealthReads(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, set enrollment.HolidayRegistrationRecordSet, withHealth bool) error {
	if !withHealth {
		return nil
	}
	ids := make([]int64, len(set.Records))
	for i, record := range set.Records {
		ids[i] = record.Detail.ID
	}
	if err := r.store.AppendHolidayHealthReads(ctx, reviewer.OrganizationID, periodID, ids,
		`{"format":"`+holidayRecordFormat+`"}`, reviewer.AccountID); err != nil {
		return fmt.Errorf("enrollment: record holiday registration health: %w", err)
	}
	return nil
}

// holidayRecordParts are the reads of a period the records share.
type holidayRecordParts struct {
	contacts map[int64][]enrollment.HolidayRegistrationContact
	answers  map[int64]enrollment.HolidayRegistrationAnswers
	forms    map[int]enrollment.HolidayPeriodForm
	criteria []enrollment.HolidayCriterion
	stated   map[int64][]enrollment.HolidayCriterionAnswer
	health   map[int64]enrollment.HolidaySealedRegistrationHealth
}

// read reads the registrations of the period, or the one registrationID
// names, that match the filter, each with its contacts, answers, criteria
// and consent texts, and with its health information when asked.
func (r *HolidayRegistrationRecords) read(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, filter enrollment.HolidayRegistrationFilter, withHealth bool) (enrollment.HolidayRegistrationRecordSet, error) {
	organizationID := reviewer.OrganizationID
	period, err := r.review.period(ctx, organizationID, periodID, false)
	if err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	details, overrides, err := r.review.store.ReadHolidayReview(ctx, organizationID, periodID, registrationID)
	if err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, fmt.Errorf("enrollment: read holiday registrations: %w", err)
	}
	if err := r.review.completeWithLunch(ctx, period, details, overrides); err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	set := enrollment.HolidayRegistrationRecordSet{Period: period, Records: []enrollment.HolidayRegistrationRecord{}}
	if len(details) == 0 {
		return set, nil
	}
	parts, err := r.parts(ctx, organizationID, period, registrationID, withHealth)
	if err != nil {
		return enrollment.HolidayRegistrationRecordSet{}, err
	}
	for _, detail := range details {
		if !holidayRegistrationMatches(detail.HolidayRegistrationSummary, filter) {
			continue
		}
		record, err := r.record(organizationID, period, detail, parts, withHealth)
		if err != nil {
			return enrollment.HolidayRegistrationRecordSet{}, err
		}
		set.Records = append(set.Records, record)
	}
	return set, nil
}

func (r *HolidayRegistrationRecords) parts(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, registrationID int64, withHealth bool) (holidayRecordParts, error) {
	var parts holidayRecordParts
	var err error
	if parts.contacts, err = r.store.ReadHolidayRecordContacts(ctx, organizationID, period.ID, registrationID); err != nil {
		return parts, fmt.Errorf("enrollment: read holiday registration contacts: %w", err)
	}
	if parts.answers, err = r.review.store.ReadHolidayAnswers(ctx, organizationID, period.ID, registrationID); err != nil {
		return parts, fmt.Errorf("enrollment: read holiday registration answers: %w", err)
	}
	if parts.forms, err = r.review.forms.HolidayPeriodForms(ctx, organizationID, period.ID, nil); err != nil {
		return parts, fmt.Errorf("enrollment: read holiday form versions: %w", err)
	}
	if err := r.criteria(ctx, organizationID, period, registrationID, &parts); err != nil {
		return parts, err
	}
	if withHealth {
		if parts.health, err = r.store.ReadHolidayRecordHealth(ctx, organizationID, period.ID, registrationID); err != nil {
			return parts, fmt.Errorf("enrollment: read holiday registration health: %w", err)
		}
	}
	return parts, nil
}

// criteria reads the current and answered historical criteria and the
// families' answers of a period by criteria; a period by arrival has none.
func (r *HolidayRegistrationRecords) criteria(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, registrationID int64, parts *holidayRecordParts) error {
	if period.AllocationMode != enrollment.HolidayAllocationCriteria {
		return nil
	}
	all, err := r.store.ReadHolidayRecordCriteria(ctx, organizationID, period.ID, registrationID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	answers, err := r.review.store.ReadHolidayCriterionAnswers(ctx, organizationID, period.ID, registrationID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday criterion answers: %w", err)
	}
	parts.criteria, parts.stated = all, map[int64][]enrollment.HolidayCriterionAnswer{}
	for _, answer := range answers {
		parts.stated[answer.RegistrationID] = append(parts.stated[answer.RegistrationID], answer)
	}
	return nil
}

func (r *HolidayRegistrationRecords) record(organizationID int64, period enrollment.HolidayPeriod, detail enrollment.HolidayRegistrationDetail, parts holidayRecordParts, withHealth bool) (enrollment.HolidayRegistrationRecord, error) {
	answers, ok := parts.answers[detail.ID]
	if !ok {
		return enrollment.HolidayRegistrationRecord{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	form, ok := parts.forms[answers.FormVersion]
	if !ok {
		return enrollment.HolidayRegistrationRecord{}, fmt.Errorf("enrollment: holiday registration %d keeps the missing form version %d", detail.ID, answers.FormVersion)
	}
	detail.Form, detail.LegalAcceptances = &form, answers.LegalAcceptances
	detail.CustomData, detail.ChildCustomData = answers.CustomData, answers.Children
	detail.Contacts = parts.contacts[detail.ID]
	if texts, ok := enrollment.HolidayConsentTextsOf(detail.ConsentVersion, detail.RetentionDays); ok {
		detail.ConsentTexts = &texts
	}
	detail.Criteria = holidayRecordCriteria(detail, parts)
	record := enrollment.HolidayRegistrationRecord{Detail: detail}
	if !withHealth {
		return record, nil
	}
	health, err := openHolidayRegistrationHealth(r.review.health, organizationID, detail.ID, parts.health[detail.ID])
	if err != nil {
		return enrollment.HolidayRegistrationRecord{}, err
	}
	record.Health = &health
	return record, nil
}

// holidayRecordCriteria names current criteria and answered historical
// criteria: the family's once, a child criterion per child.
func holidayRecordCriteria(detail enrollment.HolidayRegistrationDetail, parts holidayRecordParts) []enrollment.HolidayRegistrationCriterion {
	criteria := []enrollment.HolidayRegistrationCriterion{}
	add := func(criterion enrollment.HolidayCriterion, childID *int64) {
		answer := holidayRegistrationCriterion(criterion, childID, parts.stated[detail.ID])
		if criterion.Active || answer.Stated != enrollment.HolidayCriterionUnknown || answer.Checked != nil {
			criteria = append(criteria, answer)
		}
	}
	for _, criterion := range parts.criteria {
		if criterion.Scope == enrollment.HolidayCriterionFamily {
			add(criterion, nil)
			continue
		}
		for _, child := range detail.Children {
			add(criterion, &child.ID)
		}
	}
	return criteria
}
