package enrollmenthttp_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The files parents attach to an enrollment (#4783): upload, claim on
// submit, staff read with access log, retention and object cleanup.

const (
	uploadStufeKey    = "einkommen_stufe"
	uploadNachweisKey = "einkommen_nachweis"
)

type uploadFlowEnv struct {
	*requestTestEnv
	intake  enrollmentNative.UploadIntake
	uploads *enrollmentNative.RequestUploads
	access  *recordingAccessLog
	phaseID int64
}

type recordingAccessLog struct {
	entries []enrollmentNative.UploadAccessEntry
	fail    error
}

func (l *recordingAccessLog) RecordUploadAccess(_ context.Context, entry enrollmentNative.UploadAccessEntry) error {
	if l.fail != nil {
		return l.fail
	}
	l.entries = append(l.entries, entry)
	return nil
}

func setupUploadFlow(t *testing.T) *uploadFlowEnv {
	t.Helper()
	env := setupRequestTest(t)
	ctx := testpkg.Ctx(t)
	schemaSvc := NewTestFormSchemas(env.repos.Enrollment())
	schema, err := schemaSvc.CreateSchema(ctx, "Einkommen "+t.Name(), []enrollmentNative.FormField{
		{Key: uploadStufeKey, Label: "Einkommensstufe", Type: enrollmentNative.FormFieldSelect, Required: true, SortOrder: 0,
			Options: []enrollmentNative.FormFieldOption{{Label: "Voller Beitrag", Value: "voll"}, {Label: "Ermäßigt Stufe 1", Value: "stufe_1"}}},
		{Key: uploadNachweisKey, Label: "Einkommensnachweis", Type: enrollmentNative.FormFieldFile, Required: true, SortOrder: 1,
			FileOptions: &enrollmentNative.FormFileOptions{AllowedTypes: []string{"pdf", "jpg"}, MaxSizeMB: 2, Multiple: true},
			VisibleWhen: &enrollmentNative.VisibilityCondition{Source: enrollmentNative.ConditionSourceField, Field: uploadStufeKey, Operator: enrollmentNative.ConditionOpNotEquals, Value: "voll"}},
	}, env.creatorID)
	require.NoError(t, err)
	phase := &enrollmentNative.Phase{
		Name: "upload-" + t.Name(), Kind: enrollmentNative.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentNative.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   enrollmentNative.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true, FormSchemaID: &schema.ID, CareOverflowMode: enrollmentNative.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentAPI.InsertOwnerPhaseForTest(ctx, env.repos.Enrollment(), phase))
	env.settings.intValues["enrollment.upload_retention_days"] = 90
	access := &recordingAccessLog{}
	return &uploadFlowEnv{
		requestTestEnv: env,
		intake:         owners.NewEnrollmentIntake(env.config),
		uploads: enrollmentNative.NewRequestUploads(enrollmentNative.RequestUploadDependencies{
			Records: env.repos.Enrollment(), AccessLog: access, Settings: env.settings,
			NotFound: func(err error) bool { return errors.Is(err, sql.ErrNoRows) }, Logger: slog.Default(),
		}),
		access:  access,
		phaseID: phase.ID,
	}
}

func (e *uploadFlowEnv) inTenant(t *testing.T, fn func(context.Context) error) error {
	t.Helper()
	id, err := tenant.NewTenantID(testpkg.Tenant(t))
	require.NoError(t, err)
	return tenant.WithinTenant(testpkg.Ctx(t), id, fn)
}

func (e *uploadFlowEnv) prepare(t *testing.T, input enrollmentNative.UploadInput) (*enrollmentNative.PreparedUpload, error) {
	t.Helper()
	input.TenantID = testpkg.Tenant(t)
	if input.PhaseID == 0 {
		input.PhaseID = e.phaseID
	}
	var prepared *enrollmentNative.PreparedUpload
	err := e.inTenant(t, func(ctx context.Context) error {
		var prepareErr error
		prepared, prepareErr = e.intake.PrepareUpload(ctx, input)
		return prepareErr
	})
	return prepared, err
}

func pdfUpload(name string) enrollmentNative.UploadInput {
	return enrollmentNative.UploadInput{
		FieldKey: uploadNachweisKey, Filename: name, ContentType: "application/pdf", Extension: ".pdf",
		SizeBytes: 1024, RemoteIP: "198.51.100.7",
	}
}

func (e *uploadFlowEnv) submission(t *testing.T, answers map[string]any) enrollmentAPI.SubmitRequest {
	req := validSubmission(t, e.phaseID)
	req.CustomData = answers
	return req
}

func TestEnrollmentUploads_PrepareChecksFieldTypeAndSize(t *testing.T) {
	t.Parallel()
	env := setupUploadFlow(t)

	prepared, err := env.prepare(t, pdfUpload("Bescheid.pdf"))
	require.NoError(t, err)
	assert.NotEmpty(t, prepared.Token)
	assert.Equal(t, "Bescheid.pdf", prepared.Upload.Filename)
	assert.NotContains(t, prepared.Upload.StoredName, "Bescheid", "the display name never reaches the store")
	assert.NotEqual(t, prepared.Token, prepared.Upload.TokenHash, "only the token hash is stored")

	png := pdfUpload("Foto.png")
	png.ContentType, png.Extension = "image/png", ".png"
	_, err = env.prepare(t, png)
	var rejection *enrollmentNative.UploadRejection
	require.ErrorAs(t, err, &rejection, "png is not allowed on this field")
	assert.Equal(t, 2, rejection.MaxMB)
	assert.Equal(t, []string{"pdf", "jpg"}, rejection.AllowedTypes)

	large := pdfUpload("Gross.pdf")
	large.SizeBytes = 2*1024*1024 + 1
	_, err = env.prepare(t, large)
	require.ErrorAs(t, err, &rejection, "the field limit is 2 MB")

	unknown := pdfUpload("Bescheid.pdf")
	unknown.FieldKey = uploadStufeKey
	_, err = env.prepare(t, unknown)
	require.ErrorIs(t, err, enrollmentNative.ErrUploadFieldNotFound, "a select field takes no files")
}

func TestEnrollmentUploads_SubmitClaimsUploadAndRefusesReuse(t *testing.T) {
	t.Parallel()
	env := setupUploadFlow(t)
	ctx := testpkg.Ctx(t)

	prepared, err := env.prepare(t, pdfUpload("Bescheid.pdf"))
	require.NoError(t, err)
	answers := map[string]any{
		uploadStufeKey:    "stufe_1",
		uploadNachweisKey: []any{map[string]any{"token": prepared.Token, "filename": "manipuliert.exe", "size_bytes": 1}},
	}
	result, err := env.svc.Submit(ctx, env.submission(t, answers))
	require.NoError(t, err)

	stored, ok := result.Request.CustomData[uploadNachweisKey].([]any)
	require.True(t, ok, "the stored answer is a list: %#v", result.Request.CustomData[uploadNachweisKey])
	require.Len(t, stored, 1)
	item := stored[0].(map[string]any)
	assert.Equal(t, strconv.FormatInt(prepared.Upload.ID, 10), item["upload_id"])
	assert.Equal(t, "Bescheid.pdf", item["filename"], "metadata comes from the row, not from the client")
	assert.NotContains(t, item, "token", "the stored answer never keeps the token")

	var listed []enrollmentNative.RequestUpload
	require.NoError(t, env.inTenant(t, func(txCtx context.Context) error {
		listed, err = env.uploads.ListRequestUploads(txCtx, result.Request.ID)
		return err
	}))
	require.Len(t, listed, 1)
	assert.Equal(t, prepared.Upload.ID, listed[0].ID)

	again := env.submission(t, answers)
	again.GuardianEmail = "zweite@example.com"
	again.Children[0].FirstName = "Mira"
	_, err = env.svc.Submit(ctx, again)
	var rejection *enrollmentNative.InvalidInputError
	require.ErrorAs(t, err, &rejection, "a claimed token cannot be claimed again")
	assert.Equal(t, enrollmentNative.CodeUploadExpired, rejection.Code)
}

func TestEnrollmentUploads_RequiredFileFollowsCondition(t *testing.T) {
	t.Parallel()
	env := setupUploadFlow(t)
	ctx := testpkg.Ctx(t)

	_, err := env.svc.Submit(ctx, env.submission(t, map[string]any{uploadStufeKey: "stufe_1"}))
	var rejection *enrollmentNative.InvalidInputError
	require.ErrorAs(t, err, &rejection, "a reduced level needs the proof")
	assert.Equal(t, enrollmentNative.CodeFieldRequired, rejection.Code)

	result, err := env.svc.Submit(ctx, env.submission(t, map[string]any{uploadStufeKey: "voll"}))
	require.NoError(t, err, "the full fee needs no proof")
	assert.NotContains(t, result.Request.CustomData, uploadNachweisKey)
}

func TestEnrollmentUploads_OpenRecordsAccessOrRefuses(t *testing.T) {
	t.Parallel()
	env := setupUploadFlow(t)
	ctx := testpkg.Ctx(t)

	prepared, err := env.prepare(t, pdfUpload("Bescheid.pdf"))
	require.NoError(t, err)
	result, err := env.svc.Submit(ctx, env.submission(t, map[string]any{
		uploadStufeKey: "stufe_1", uploadNachweisKey: []any{map[string]any{"token": prepared.Token}},
	}))
	require.NoError(t, err)

	access := enrollmentNative.UploadAccess{RequestID: result.Request.ID, UploadID: prepared.Upload.ID, ActorAccountID: env.creatorID, ActorRole: "admin"}
	var opened *enrollmentNative.RequestUpload
	require.NoError(t, env.inTenant(t, func(txCtx context.Context) error {
		opened, err = env.uploads.OpenRequestUpload(txCtx, access)
		return err
	}))
	assert.Equal(t, prepared.Upload.StoredName, opened.StoredName)
	require.Len(t, env.access.entries, 1)
	entry := env.access.entries[0]
	assert.Equal(t, env.creatorID, entry.ActorAccountID)
	assert.Equal(t, prepared.Upload.ID, entry.UploadID)
	assert.Equal(t, result.Request.ID, entry.RequestID)
	assert.Equal(t, uploadNachweisKey, entry.FieldKey)

	env.access.fail = errors.New("audit down")
	err = env.inTenant(t, func(txCtx context.Context) error {
		_, openErr := env.uploads.OpenRequestUpload(txCtx, access)
		return openErr
	})
	require.Error(t, err, "without an access record the file is not handed out")

	env.access.fail = nil
	wrong := access
	wrong.RequestID = result.Request.ID + 1_000_000
	err = env.inTenant(t, func(txCtx context.Context) error {
		_, openErr := env.uploads.OpenRequestUpload(txCtx, wrong)
		return openErr
	})
	require.ErrorIs(t, err, enrollmentNative.ErrUploadNotFound, "an upload is only reachable through its own request")
}

func TestEnrollmentUploads_RetentionDeletesAndSweepRemovesObjects(t *testing.T) {
	t.Parallel()
	env := setupUploadFlow(t)
	ctx := testpkg.Ctx(t)

	claimed, err := env.prepare(t, pdfUpload("Bescheid.pdf"))
	require.NoError(t, err)
	unclaimed, err := env.prepare(t, pdfUpload("Liegengeblieben.pdf"))
	require.NoError(t, err)
	fresh, err := env.prepare(t, pdfUpload("Frisch.pdf"))
	require.NoError(t, err)
	result, err := env.svc.Submit(ctx, env.submission(t, map[string]any{
		uploadStufeKey: "stufe_1", uploadNachweisKey: []any{map[string]any{"token": claimed.Token}},
	}))
	require.NoError(t, err)

	// Nothing is due yet: the request is open and the unclaimed uploads are new.
	var deleted, days int
	require.NoError(t, env.inTenant(t, func(txCtx context.Context) error {
		deleted, days, err = env.uploads.DeleteExpiredUploads(txCtx)
		return err
	}))
	assert.Equal(t, 0, deleted)
	assert.Equal(t, 90, days)

	decidedAt := time.Now().Add(-91 * 24 * time.Hour)
	_, err = env.db.NewRaw(`UPDATE enrollment.request_children SET status = 'rejected', reviewed_at = ? WHERE request_id = ?`, decidedAt, result.Request.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = env.db.NewRaw(`UPDATE enrollment.request_uploads SET created_at = ? WHERE id = ?`, time.Now().Add(-25*time.Hour), unclaimed.Upload.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, env.inTenant(t, func(txCtx context.Context) error {
		deleted, _, err = env.uploads.DeleteExpiredUploads(txCtx)
		return err
	}))
	assert.Equal(t, 2, deleted, "the decided request's proof and the expired unclaimed upload go; the fresh one stays")

	var removed []string
	remove := func(_ context.Context, tenantID int64, storedName string) error {
		assert.Equal(t, testpkg.Tenant(t), tenantID)
		removed = append(removed, storedName)
		return nil
	}
	var swept int
	require.NoError(t, env.inTenant(t, func(txCtx context.Context) error {
		swept, err = env.uploads.SweepUploadObjects(txCtx, remove)
		return err
	}))
	assert.Equal(t, 2, swept)
	assert.True(t, slices.Contains(removed, claimed.Upload.StoredName))
	assert.True(t, slices.Contains(removed, unclaimed.Upload.StoredName))
	assert.False(t, slices.Contains(removed, fresh.Upload.StoredName))

	require.NoError(t, env.inTenant(t, func(txCtx context.Context) error {
		swept, err = env.uploads.SweepUploadObjects(txCtx, remove)
		return err
	}))
	assert.Equal(t, 0, swept, "settled intents are not swept twice")
}
