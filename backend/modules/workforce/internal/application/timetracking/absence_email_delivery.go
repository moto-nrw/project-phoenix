package timetracking

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

// absenceRequestReceivedTemplate is the mail an approver gets for a new
// absence request; every other absence mail answers the requester.
const absenceRequestReceivedTemplate = "absence-request-received.html"

// TemplateData is the content the message's template renders: the request
// mail carries the requester, the note and the previous question, every
// other absence mail the decision note.
func (m AbsenceEmailMessage) TemplateData() map[string]any {
	content := map[string]any{
		"FirstName":        m.Content.FirstName,
		"LastName":         m.Content.LastName,
		"AbsenceTypeLabel": m.Content.AbsenceTypeLabel,
		"DateRange":        m.Content.DateRange,
		"LinkURL":          m.Content.LinkURL,
		"LogoURL":          m.Content.LogoURL,
	}
	if m.Template == absenceRequestReceivedTemplate {
		content["RequesterName"] = m.Content.RequesterName
		content["Note"] = m.Content.Note
		content["PreviousQuestion"] = m.Content.PreviousQuestion
	} else {
		content["DecisionNote"] = m.Content.DecisionNote
	}
	return content
}
