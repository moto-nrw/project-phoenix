package active

import "context"

type AbsenceEmailAddress struct{ Name, Address string }
type AbsenceEmailContent struct {
	FirstName, LastName, RequesterName   string
	AbsenceTypeLabel, DateRange          string
	Note, PreviousQuestion, DecisionNote string
	LinkURL, LogoURL                     string
}
type AbsenceEmailMessage struct {
	TenantID, ReferenceID int64
	Type, Recipient       string
	To                    AbsenceEmailAddress
	Subject, Template     string
	Content               AbsenceEmailContent
}
type absenceEmailDispatcher interface {
	Dispatch(context.Context, AbsenceEmailMessage)
}
