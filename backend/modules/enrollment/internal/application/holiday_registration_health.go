package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// HolidayRegistrationHealth opens the health information of one registration
// of the period for the Träger-Büro (#4722): the family's answers to its
// health questions and each child's health fields and answers, every child
// in its order, also one without health information. The read is recorded
// in the registration's audit trail in the same transaction; without the
// entry nothing is answered. The route admits only traeger_health:read.
func (h *HolidayRegistrationReview) HolidayRegistrationHealth(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64) (enrollment.HolidayRegistrationHealthDetail, error) {
	if err := holidayReviewKey(reviewer, periodID, registrationID); err != nil {
		return enrollment.HolidayRegistrationHealthDetail{}, err
	}
	if _, err := h.period(ctx, reviewer.OrganizationID, periodID, false); err != nil {
		return enrollment.HolidayRegistrationHealthDetail{}, err
	}
	sealed, found, err := h.store.ReadHolidaySealedHealth(ctx, reviewer.OrganizationID, periodID, registrationID)
	if err != nil {
		return enrollment.HolidayRegistrationHealthDetail{}, fmt.Errorf("enrollment: read holiday registration health: %w", err)
	}
	if !found {
		return enrollment.HolidayRegistrationHealthDetail{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	detail, err := openHolidayRegistrationHealth(h.health, reviewer.OrganizationID, registrationID, sealed)
	if err != nil {
		return enrollment.HolidayRegistrationHealthDetail{}, err
	}
	if err := h.audit(ctx, reviewer, periodID, registrationID, enrollment.HolidayAuditHealth, nil); err != nil {
		return enrollment.HolidayRegistrationHealthDetail{}, err
	}
	return detail, nil
}

// openHolidayRegistrationHealth opens the sealed health information of one
// registration: the family's answers and each child's fields and answers,
// every child in its order, also one without health information.
func openHolidayRegistrationHealth(cipher enrollment.HolidayHealthCipher, organizationID, registrationID int64, sealed enrollment.HolidaySealedRegistrationHealth) (enrollment.HolidayRegistrationHealthDetail, error) {
	family, err := openHolidayHealthRecord(cipher, sealed.Family, enrollment.HolidayFamilyHealthContext(organizationID, registrationID))
	if err != nil {
		return enrollment.HolidayRegistrationHealthDetail{}, err
	}
	detail := enrollment.HolidayRegistrationHealthDetail{
		CustomData: answersOrEmptyObject(family.Answers),
		Children:   make([]enrollment.HolidayRegistrationHealth, len(sealed.Children)),
	}
	for i, child := range sealed.Children {
		record, err := openHolidayHealthRecord(cipher, child.Sealed,
			enrollment.HolidayChildHealthContext(organizationID, registrationID, child.Position))
		if err != nil {
			return enrollment.HolidayRegistrationHealthDetail{}, err
		}
		detail.Children[i] = enrollment.HolidayRegistrationHealth{
			ChildID: child.ChildID, Allergies: record.Allergies, Medication: record.Medication,
			HealthNotes: record.HealthNotes, CustomData: answersOrEmptyObject(record.Answers),
		}
	}
	return detail, nil
}

// answersOrEmptyObject keeps a JSON answer object an object.
func answersOrEmptyObject(answers enrollment.HolidayAnswers) enrollment.HolidayAnswers {
	if answers == nil {
		return enrollment.HolidayAnswers{}
	}
	return answers
}
