package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// contractMailTemplate is the mail that carries a contract.
const contractMailTemplate = "care-contract.html"

// RenderContractMail turns the payload of a claimed contract mail into the
// mail the worker sends: the Träger is the sender and the stored PDF the
// attachment (ADR 0123 point 2, ADR 0153).
func RenderContractMail(_ context.Context, payload []byte) (carecontract.RenderedContractMail, error) {
	var mail carecontract.ContractMail
	if err := json.Unmarshal(payload, &mail); err != nil {
		return carecontract.RenderedContractMail{}, fmt.Errorf("%s payload is not a contract mail: %w", carecontract.MailKindContract, err)
	}
	// A reminder carries only the link; the contract and the confirmation
	// carry the PDF (#4679, signature_mail.go).
	if mail.Purpose != carecontract.MailPurposeReminder && (len(mail.PDF) == 0 || strings.TrimSpace(mail.Filename) == "") {
		return carecontract.RenderedContractMail{}, errors.New(carecontract.MailKindContract + " payload has no contract")
	}
	// The subject names no child: the mailers log it at Info level, where no
	// child's name belongs. The body names the child.
	subject, template, err := contractMailKind(mail.Purpose)
	if err != nil {
		return carecontract.RenderedContractMail{}, err
	}
	if mail.Concluded && template == contractMailTemplate {
		// Concluded at the acceptance (Weg b, #4680): the parents receive it
		// for their records and sign nothing.
		subject = "Ihr Betreuungsvertrag ist abgeschlossen"
	}
	return carecontract.RenderedContractMail{
		SenderName: strings.TrimSpace(mail.TraegerName),
		Subject:    subject,
		Template:   template,
		Content: carecontract.ContractMailContent{
			GuardianName: mail.GuardianName, TraegerName: mail.TraegerName,
			ChildName: mail.ChildName, Modules: mail.Modules, Concluded: mail.Concluded,
			SigningURL: mail.SigningURL, LinkExpiresOn: mail.LinkExpiresOn, SignedAt: mail.SignedAt,
			SignerName: mail.SignerName, DocumentSHA256: mail.DocumentSHA256, EarlyStart: mail.EarlyStart,
			WithdrawalEndsOn: mail.WithdrawalEndsOn, WithdrawalNotice: mail.WithdrawalNotice,
		},
		Filename: mail.Filename,
		PDF:      mail.PDF,
	}, nil
}
