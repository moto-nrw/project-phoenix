package application

import (
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The receipt mail of a submitted holiday registration (#4661, #4699,
// #5626): the booked weeks per child and the parent's copy of the
// declarations.

// holidayReceiptMail lists each child with one line per booked week: dates,
// host school and time model, the weeks with a place apart from those on the
// waiting list (#4699). It names no date of birth and no health information.
func holidayReceiptMail(traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod, input enrollment.HolidayRegistrationInput,
	placements []enrollment.HolidayReceiptPlacement) enrollment.HolidayRegistrationMail {
	line := holidayWeekLines(traeger, period)
	waitlisted := map[[2]int64]bool{}
	for _, placement := range placements {
		if placement.Placement == enrollment.HolidayPlacementWaitlisted {
			waitlisted[[2]int64{int64(placement.ChildPosition), placement.WeekID}] = true
		}
	}
	children := make([]enrollment.HolidayRegistrationMailChild, 0, len(input.Children))
	for i, child := range input.Children {
		children = append(children, holidayReceiptChild(child, period, line, func(weekID int64) bool {
			return waitlisted[[2]int64{int64(i + 1), weekID}]
		}))
	}
	return enrollment.HolidayRegistrationMail{
		Recipient: input.Guardian.Email, GuardianName: input.Guardian.FirstName + " " + input.Guardian.LastName,
		TraegerName: traeger.Name, PeriodName: period.Name, Binding: period.Binding, Children: children,
	}
}

// holidayMailDeclaration is the parent's copy of the declarations (#5626):
// the typed name, when it was given, the version of the texts and each
// declaration given in its words, then each confirmed notice of the Träger
// from the form version the registration keeps. The texts name no health
// information of the children.
func holidayMailDeclaration(period enrollment.HolidayPeriod, form enrollment.HolidayPeriodForm, input enrollment.HolidayRegistrationInput, givenAt time.Time) *enrollment.HolidayRegistrationMailDeclaration {
	texts, ok := enrollment.HolidayConsentTextsOf(input.ConsentVersion)
	if !ok {
		return nil
	}
	statements := []enrollment.HolidayDeclarationStatement{{Title: "Datenschutz", Text: texts.PrivacyConsent}}
	if input.HealthConsent {
		statements = append(statements, enrollment.HolidayDeclarationStatement{Title: "Gesundheitsangaben", Text: texts.HealthConsent})
	}
	if period.Binding && input.BindingConfirmed {
		statements = append(statements, enrollment.HolidayDeclarationStatement{Title: "Verbindliche Anmeldung", Text: texts.BindingConfirmed})
	}
	statements = append(statements, enrollment.HolidayDeclarationStatement{Title: "Sorgerecht", Text: texts.CustodyConfirmed})
	for _, block := range form.LegalBlocks {
		if block.Enabled && input.LegalAcceptances[block.Key] {
			statements = append(statements, enrollment.HolidayDeclarationStatement{
				Title: block.Title, Text: block.Label, Detail: strings.TrimSpace(block.Text),
			})
		}
	}
	return &enrollment.HolidayRegistrationMailDeclaration{
		Name: input.DeclarationName, GivenAt: givenAt, ConsentVersion: texts.Version, Statements: statements,
	}
}

// holidayReceiptChild lists the booked weeks of a child in the order of the
// period, those with a place apart from those on the waiting list.
func holidayReceiptChild(child enrollment.HolidayChildInput, period enrollment.HolidayPeriod, line func(weekID, timeModelID int64) string,
	waitlisted func(weekID int64) bool) enrollment.HolidayRegistrationMailChild {
	mailChild := enrollment.HolidayRegistrationMailChild{
		Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == 0, Weeks: []string{},
	}
	booked := map[int64]int64{}
	for _, booking := range child.Bookings {
		booked[booking.WeekID] = booking.TimeModelID
	}
	for _, week := range period.Weeks {
		model, ok := booked[week.ID]
		switch {
		case !ok:
		case waitlisted(week.ID):
			mailChild.WaitlistWeeks = append(mailChild.WaitlistWeeks, line(week.ID, model))
		default:
			mailChild.Weeks = append(mailChild.Weeks, line(week.ID, model))
		}
	}
	return mailChild
}
