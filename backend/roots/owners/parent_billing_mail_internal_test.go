package owners

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
)

// The worker sends the mail of a confirmation letter as a mail of the
// Träger with the stored PDF attached (#4670); without a letter in its
// payload the mail fails instead of going out empty.
func TestParentBillingLetterMailCarriesTheLetter(t *testing.T) {
	t.Parallel()

	render := parentBillingLetterMailRenderer(parentBillingCompose.NewLetterMailRenderer(), email.NewEmail("moto", "noreply@moto.example"))
	pdf := []byte("%PDF-1.4 Bestätigung FE-0001")
	payload, err := json.Marshal(parentbilling.BillingLetterMail{
		DocumentID: 7, GuardianName: "Eva Muster", TraegerName: "Ganztag im Quartier e.V.", PeriodName: "Herbstferien 2026",
		Number: "FE-0001", Filename: "bestaetigung-fe-0001.pdf", PDF: pdf,
	})
	require.NoError(t, err)
	message, err := render.Render(context.Background(), &deliveryCompose.MailIntent{Kind: parentbilling.MailKindBillingLetter, PayloadJSON: payload})
	require.NoError(t, err)
	assert.Equal(t, email.NewEmail("Ganztag im Quartier e.V.", "noreply@moto.example"), message.From, "the Träger sends from the central address")
	assert.Equal(t, "Bestätigung und Rechnung: Herbstferien 2026", message.Subject)
	assert.Equal(t, "billing-letter.html", message.Template)
	assert.Equal(t, []email.Attachment{{Filename: "bestaetigung-fe-0001.pdf", ContentType: "application/pdf", Data: pdf}}, message.Attachments)
	assert.Equal(t, "FE-0001", message.Content.(map[string]any)["Number"])

	empty, err := json.Marshal(parentbilling.BillingLetterMail{Number: "FE-0002"})
	require.NoError(t, err)
	_, err = render.Render(context.Background(), &deliveryCompose.MailIntent{Kind: parentbilling.MailKindBillingLetter, PayloadJSON: empty})
	assert.Error(t, err)
}
