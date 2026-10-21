package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestNoticeAttachmentsSurvivePublishingAndVersionReplacement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "notice-attachments")
	owner := compose.New()
	forms := compose.NewFormSchemas(owner, nil, nil)
	prefix := fmt.Sprintf("%s%d_", enrollment.EnrollmentFormLegalDocumentUploadPrefix, testpkg.Tenant(t))
	original := []enrollment.FormAttachment{
		{Label: "Schulstart", DocumentURL: prefix + "start.pdf"},
		{Label: "Abholen", DocumentURL: prefix + "pickup.pdf"},
	}
	fields := []enrollment.FormField{{Key: "notice", Type: enrollment.FormFieldInfo, Label: "Hinweise", Content: "Bitte lesen Sie die Informationen.", Attachments: original}}
	first, err := forms.CreateSchema(ctx, "Informationen", fields, actor.ID)
	require.NoError(t, err)
	saved, err := testpkg.TenantQuery1(t, ctx, db, owner.Schema, first.ID)
	require.NoError(t, err)
	require.Equal(t, original, saved.Fields[0].Attachments)
	public := enrollment.PublicFormFields(saved.Fields)
	require.Equal(t, []enrollment.FormAttachment{
		{Label: "Schulstart", DocumentURL: fmt.Sprintf("%s%d_start.pdf", enrollment.EnrollmentFormLegalDocumentPublicPrefix, testpkg.Tenant(t))},
		{Label: "Abholen", DocumentURL: fmt.Sprintf("%s%d_pickup.pdf", enrollment.EnrollmentFormLegalDocumentPublicPrefix, testpkg.Tenant(t))},
	}, public[0].Attachments)
	require.False(t, public[0].Required, "an information sheet must not require consent")
	require.Equal(t, original, saved.Fields[0].Attachments, "public URL conversion must not mutate stored evidence")

	fields[0].Attachments = []enrollment.FormAttachment{{Label: "Schulstart", DocumentURL: prefix + "replacement.pdf"}}
	second, err := forms.UpdateSchema(ctx, first.ID, fields, actor.ID)
	require.NoError(t, err)
	require.Equal(t, 2, second.Version)
	historical, err := testpkg.TenantQuery1(t, ctx, db, forms.SchemaVersion, first.ID)
	require.NoError(t, err)
	require.Equal(t, original, historical.Fields[0].Attachments, "replacement/removal must not rewrite historical forms")
	latest, err := testpkg.TenantQuery1(t, ctx, db, forms.SchemaVersion, second.ID)
	require.NoError(t, err)
	require.Equal(t, fields[0].Attachments, latest.Fields[0].Attachments)
	for _, document := range append(original, fields[0].Attachments...) {
		referenced, readErr := testpkg.TenantQuery2(t, ctx, db, owner.SchemaReferencesLegalDocument, document.DocumentURL, enrollment.PublicEnrollmentFormLegalDocumentURL(document.DocumentURL))
		require.NoError(t, readErr)
		require.True(t, referenced, "even files removed from the latest version must remain protected: %s", document.DocumentURL)
	}
	referenced, err := testpkg.TenantQuery2(t, ctx, db, owner.SchemaReferencesLegalDocument, prefix+"draft.pdf", enrollment.PublicEnrollmentFormLegalDocumentURL(prefix+"draft.pdf"))
	require.NoError(t, err)
	require.False(t, referenced, "discarded unpublished files must remain eligible for cleanup")
}

func TestNoticeAttachmentsRejectInvalidPublicationWithoutCreatingVersion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	foreignTenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	owner := compose.New()
	forms := compose.NewFormSchemas(owner, nil, nil)
	for _, scenario := range []struct {
		name, label, url string
		fieldType        enrollment.FormFieldType
		errorField       string
	}{
		{"foreign school", "Schulstart", fmt.Sprintf("%s%d_foreign.pdf", enrollment.EnrollmentFormLegalDocumentUploadPrefix, foreignTenantID), enrollment.FormFieldInfo, "fields.0.attachments.0.document_url"},
		{"external URL", "Schulstart", "https://example.com/file.pdf", enrollment.FormFieldInfo, "fields.0.attachments.0.document_url"},
		{"missing name", " ", "", enrollment.FormFieldInfo, "fields.0.attachments.0.label"},
		{"answer field", "Schulstart", "", enrollment.FormFieldText, "fields.0.attachments"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			actor := testpkg.CreateTestAccount(t, db, "invalid-attachment")
			url := scenario.url
			if url == "" {
				url = fmt.Sprintf("%s%d_valid.pdf", enrollment.EnrollmentFormLegalDocumentUploadPrefix, testpkg.Tenant(t))
			}
			_, err := forms.CreateSchema(ctx, "Informationen", []enrollment.FormField{{Key: "notice", Label: "Hinweise", Content: "Bitte lesen Sie die Informationen.", Type: scenario.fieldType, Attachments: []enrollment.FormAttachment{{Label: scenario.label, DocumentURL: url}}}}, actor.ID)
			var rejected *enrollment.InvalidInputError
			require.ErrorAs(t, err, &rejected)
			require.Equal(t, enrollment.CodeFormAttachmentInvalid, rejected.Code)
			require.Equal(t, scenario.errorField, rejected.Field)
			versions, err := testpkg.TenantQuery0(t, ctx, db, owner.SchemaVersions)
			require.NoError(t, err)
			require.Empty(t, versions, "a rejected attachment must never publish a partial version")
		})
	}
}

func TestLegalNoticeAttachmentsRemainProtectedWhenDisabledAndTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := compose.New()
	forms := compose.NewFormSchemas(owner, nil, nil)
	actor := testpkg.CreateTestAccount(t, db, "legal-notice-attachments")
	url := fmt.Sprintf("%s%d_legal.pdf", enrollment.EnrollmentFormLegalDocumentPublicPrefix, testpkg.Tenant(t))
	blocks := []enrollment.FormLegalBlock{{Key: "information", Kind: enrollment.LegalBlockKindNotice, Title: "Informationen", Enabled: false, Attachments: []enrollment.FormAttachment{{Label: "Elternbrief", DocumentURL: url}}}}
	saved, err := forms.CreateSchemaWithLegal(ctx, "Hinweise", nil, actor.ID, enrollment.CoreRequirements{}, blocks)
	require.NoError(t, err)
	storedURL := enrollment.EnrollmentFormLegalDocumentUploadPrefix + fmt.Sprintf("%d_legal.pdf", testpkg.Tenant(t))
	require.Equal(t, storedURL, saved.LegalBlocks[0].Attachments[0].DocumentURL)
	referenced, err := testpkg.TenantQuery2(t, ctx, db, owner.SchemaReferencesLegalDocument, storedURL, url)
	require.NoError(t, err)
	require.True(t, referenced, "disabled blocks must retain saved document evidence")
	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		otherCtx := testpkg.Ctx(t)
		_, err := testpkg.TenantQuery1(t, otherCtx, db, forms.SchemaVersion, saved.ID)
		require.ErrorIs(t, err, enrollment.ErrFormSchemaNotFound)
		referenced, err := testpkg.TenantQuery0(t, otherCtx, db, func(txCtx context.Context) (bool, error) {
			return owner.SchemaReferencesLegalDocument(txCtx, storedURL, url)
		})
		require.NoError(t, err)
		require.False(t, referenced, "another school's references must not leak across the tenant boundary")
	})
}
