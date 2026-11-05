// Router-level tests for the layout of the table of amounts (#5691): the
// layout is part of a version, a change of the layout alone is a new
// version, the preview fills the sample document with address field,
// numbers and the chosen table, and a kind without a table refuses a
// layout.
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type invoiceLayoutBody struct {
	GroupByChild   bool `json:"group_by_child"`
	HideTimeModels bool `json:"hide_time_models"`
}

type documentTemplateWithLayout struct {
	documentTemplateBody
	Invoice invoiceLayoutBody `json:"invoice"`
}

type layoutVersion struct {
	Version int               `json:"version"`
	Invoice invoiceLayoutBody `json:"invoice"`
}

func TestTraegerDocumentTemplatesKeepTheInvoiceLayoutWithTheVersion(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newMasterDataOffice(t, db, router, "Träger Melle Rechnungstabelle", identityaccess.TraegerRoleAdministration)
	save := func(body documentTemplateWithLayout) layoutVersion {
		rec := traegerJSON(t, router, office.token, http.MethodPost, documentTemplatesPath+"/bestaetigung/versions", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[layoutVersion](t, rec.Body.Bytes())
	}

	first := save(documentTemplateWithLayout{documentTemplateBody: hardtInvoice})
	assert.Equal(t, invoiceLayoutBody{}, first.Invoice, "without a layout the standard table applies")
	grouped := invoiceLayoutBody{GroupByChild: true, HideTimeModels: true}
	second := save(documentTemplateWithLayout{documentTemplateBody: hardtInvoice, Invoice: grouped})
	assert.Equal(t, 2, second.Version, "a change of the layout alone is a new version")

	rec := traegerJSON(t, router, office.token, http.MethodGet, documentTemplatesPath+"/bestaetigung", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[struct {
		Current  layoutVersion   `json:"current"`
		Versions []layoutVersion `json:"versions"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, grouped, detail.Current.Invoice)
	require.Len(t, detail.Versions, 2)
	assert.Equal(t, invoiceLayoutBody{}, detail.Versions[1].Invoice, "the first version keeps its table")

	_, err := db.ExecContext(context.Background(), `UPDATE platform.traeger_document_templates SET invoice_hide_time_models = true
		WHERE organization_id = ? AND kind = 'bestaetigung' AND version = 1`, office.organization)
	require.Error(t, err, "the layout of a version never changes")
}

func TestTraegerDocumentTemplatePreviewShowsTheWholeLetter(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newMasterDataOffice(t, db, router, "Träger Melle Vorschau", identityaccess.TraegerRoleAdministration)

	body := documentTemplateWithLayout{documentTemplateBody: hardtInvoice, Invoice: invoiceLayoutBody{GroupByChild: true}}
	body.Body = "Ihre Debitorennummer: {{debitorennummer}}\n\n{{positionen}}"
	rec := traegerJSON(t, router, office.token, http.MethodPost, documentTemplatesPath+"/bestaetigung/preview", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[struct {
		Recipient []string `json:"recipient"`
		Facts     []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"facts"`
		documentTemplatePreview
	}](t, rec.Body.Bytes())
	assert.Equal(t, "Familie Muster", preview.Recipient[0])
	require.NotEmpty(t, preview.Facts)
	assert.Equal(t, "Rechnungsnummer", preview.Facts[0].Label)
	assert.Equal(t, "Debitorennummer", preview.Facts[1].Label)
	require.Len(t, preview.Blocks, 2)
	assert.Equal(t, "Ihre Debitorennummer: 10234", preview.Blocks[0].Text)
	assert.Equal(t, "Lina", preview.Blocks[1].Rows[0].Label, "the grouped table names the child once")

	rec = traegerJSON(t, router, office.token, http.MethodPost, documentTemplatesPath+"/kuendigung/versions",
		documentTemplateWithLayout{documentTemplateBody: documentTemplateBody{Subject: "Kündigung", Body: "Text"}, Invoice: invoiceLayoutBody{HideTimeModels: true}})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	refusal := refusalOf(t, rec)
	assert.Equal(t, "traeger.document_template_invalid", refusal.Code)
	require.Len(t, refusal.Errors, 1)
	assert.Equal(t, "invoice", refusal.Errors[0].Field)
}
