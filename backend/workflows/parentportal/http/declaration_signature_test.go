package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func signedProofFixture() *parentService.DeclarationProof {
	proof := declarationProofFixture(true)
	id := int64(55)
	proof.Submissions[0].SignatureID = &id
	proof.Submissions[0].Method = parentService.DeclarationMethodSimpleElectronicSignature
	proof.Signatures = map[int64]*parentService.DeclarationSignature{id: {
		ID: id, Method: "typed", TypedName: "Klaus Richter", PNG: []byte("\x89PNG-bytes"),
		ImageSHA256: strings.Repeat("d", 64), DocumentSHA256: strings.Repeat("b", 64),
		SignedAt: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC), IntegrityOK: true,
	}}
	return proof
}

// The guardian's proof shows the own signature as an image and names the
// signature procedure; an unsigned answer has no signature.
func TestDeclarationProofResponseCarriesTheSignature(t *testing.T) {
	t.Parallel()
	out := toDeclarationProofResponse(signedProofFixture(), time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC))
	assert.Equal(t, signatureMethodLabel, out.MethodLabel)
	require.NotNil(t, out.Submissions[0].Signature)
	assert.Equal(t, "data:image/png;base64,iVBORy1ieXRlcw==", out.Submissions[0].Signature.ImageDataURL)
	assert.Equal(t, "Klaus Richter", *out.Submissions[0].Signature.TypedName)
	assert.Nil(t, out.Submissions[1].Signature)

	unsigned := toDeclarationProofResponse(declarationProofFixture(true), time.Now())
	assert.Equal(t, methodLabel, unsigned.MethodLabel)
}

// The PDF proof prints the signature image below its label.
func TestDeclarationProofReportPrintsTheSignature(t *testing.T) {
	t.Parallel()
	doc := declarationProofReport(signedProofFixture(), time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC))
	text := proofTexts(doc)
	assert.Contains(t, text, "Verfahren="+signatureMethodLabel)
	assert.Contains(t, text, "Unterschrift=Name getippt: Klaus Richter")
	var printed []byte
	for _, section := range doc.Sections {
		for _, card := range section.Cards {
			for _, f := range card.Fields {
				if f.Label == "Unterschrift" {
					printed = f.PNG
				}
			}
		}
	}
	assert.Equal(t, []byte("\x89PNG-bytes"), printed)
}

func TestDeclarationSignatureErrorsUseTheirCodes(t *testing.T) {
	t.Parallel()
	for err, code := range map[error]string{
		parentService.ErrDeclarationSignatureRequired: "care.declaration_signature_required",
		parentService.ErrDeclarationSignatureInvalid:  "care.declaration_signature_invalid",
	} {
		w := httptest.NewRecorder()
		renderDeclarationError(w, httptest.NewRequest(http.MethodPost, "/me/news/1/declaration", nil), errors.Join(err))
		require.Equal(t, http.StatusBadRequest, w.Code)
		var body struct {
			Code string `json:"code"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, code, body.Code)
	}
}

// The feed tells the portal that agreeing needs a signature.
func TestDeclarationResponseCarriesRequiresSignature(t *testing.T) {
	t.Parallel()
	out := toDeclarationResponse(&parentService.AnnouncementFeedDeclaration{RequiresSignature: true})
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"requires_signature":true`)
}
