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
	if len(mail.PDF) == 0 || strings.TrimSpace(mail.Filename) == "" {
		return carecontract.RenderedContractMail{}, errors.New(carecontract.MailKindContract + " payload has no contract")
	}
	// The subject names no child: the mailers log it at Info level, where no
	// child's name belongs. The body names the child.
	subject := "Ihr Betreuungsvertrag"
	if mail.Concluded {
		subject = "Ihr Betreuungsvertrag ist abgeschlossen"
	}
	return carecontract.RenderedContractMail{
		SenderName: strings.TrimSpace(mail.TraegerName),
		Subject:    subject,
		Template:   contractMailTemplate,
		Content: carecontract.ContractMailContent{
			GuardianName: mail.GuardianName, TraegerName: mail.TraegerName,
			ChildName: mail.ChildName, Modules: mail.Modules, Concluded: mail.Concluded,
		},
		Filename: mail.Filename,
		PDF:      mail.PDF,
	}, nil
}
