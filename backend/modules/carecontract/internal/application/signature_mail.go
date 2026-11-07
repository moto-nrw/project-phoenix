package application

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The mails of the signature (#4679): the reminder with a new one-time link
// and the confirmation with the signed PDF and the withdrawal notice.
const (
	contractReminderMailTemplate = "care-contract-reminder.html"
	contractSignedMailTemplate   = "care-contract-signed.html"
)

// contractMailKind is the subject and template of a contract mail.
func contractMailKind(purpose string) (string, string, error) {
	switch purpose {
	case carecontract.MailPurposeContract:
		return "Ihr Betreuungsvertrag", contractMailTemplate, nil
	case carecontract.MailPurposeReminder:
		return "Erinnerung: Ihr Betreuungsvertrag wartet auf Ihre Unterschrift", contractReminderMailTemplate, nil
	case carecontract.MailPurposeSigned:
		return "Bestätigung: Ihr Betreuungsvertrag ist unterschrieben", contractSignedMailTemplate, nil
	}
	return "", "", fmt.Errorf("%s payload has the unknown purpose %q", carecontract.MailKindContract, purpose)
}
