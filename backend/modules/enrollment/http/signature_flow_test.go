package enrollmenthttp_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The signature of an enrollment (#4786) at the HTTP seam: the assembled
// router with the real owner, the real Audit Platform ledger and the real
// SQL. The proof behind the status link is reachable without an account,
// so its tenant and request binding is pinned here.

type signatureEnv struct {
	db       *bun.DB
	router   http.Handler
	tenantID int64
	phaseID  int64
	requests enrollmentAPI.RequestService
}

// setupSignatureTest builds one school whose form requires a signature.
func setupSignatureTest(t *testing.T, tenantID int64) *signatureEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	repos, err := api.NewEnrollmentTestRepositories(testpkg.NewDatabase(t, db), owners.NewTestAuditStore(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	reposCare := api.NewEnrollmentCarePlanRows(testpkg.NewDatabase(t, db))
	reposSchools := api.NewOrganizationTestRows(testpkg.NewDatabase(t, db))
	settings := stubTakeoverSettings{}

	_, author := testpkg.CreateTestPersonWithAccount(t, db, "Sina", "Signatur")
	schema, err := testpkg.TenantQuery5(t, ctx, db, NewTestFormSchemas(repos.Enrollment()).CreateSchemaWithLegal,
		"Formular mit Unterschrift "+t.Name(), []capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		}, author.ID, capability.CoreRequirements{capability.CoreRequirementSignature: true}, []capability.FormLegalBlock(nil))
	require.NoError(t, err)
	require.True(t, schema.CoreRequirements.Required(capability.CoreRequirementSignature))
	phase := &capability.Phase{
		Name: "signature-" + t.Name(), Kind: capability.PhaseKindSchoolYear,
		ServiceStartDate: capability.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   capability.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true, FormSchemaID: &schema.ID, CareOverflowMode: capability.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = tenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repos.Enrollment().InsertPhase, phase))

	offerings := enrollmentCompose.NewCareOfferingRecords(reposCare.CarePlan)
	requests := newFixtureRequestRoot(owners.NewEnrollmentIntake(owners.EnrollmentIntakeSources{
		Requests: repos.Enrollment(), ParentChanges: repos.Enrollment(), Children: repos.Enrollment(), Guardians: repos.Enrollment(),
		CareOfferingRepo: offerings, Capacity: owners.NewEnrollmentOfferingCapacity(offerings, repos.Enrollment(), settings),
		Catalog: repos.Enrollment(), SchoolRepo: capabilitySchools{schools: reposSchools.School},
		Notifications: NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: settings}, discardingOutbox{}, capabilitySchools{schools: reposSchools.School}),
		RateLimitRepo: repos.Enrollment(), OutboxEnqueuer: discardingOutbox{}, Settings: settings,
		FrontendURL: "http://localhost:3000", ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	}))
	decisions := enrollmentAPI.NewDecisionService(enrollmentCompose.PublicDecisions(owners.NewEnrollmentDecisions(enrollmentCompose.DecisionDependencies{
		Requests: repos.Enrollment(), Children: repos.Enrollment(), Guardians: repos.Enrollment(), LateInvites: repos.Enrollment(),
		Phases: repos.Enrollment(), Schemas: repos.Enrollment(), Offerings: offerings,
		ParentsURL: owners.EnrollmentDecisionParentsURL("http://parents.localhost:3000", ""),
		Settings:   owners.EnrollmentDecisionSettingValues(settings), Logger: slog.Default(),
	})))
	resource := enrollmentAPI.NewResource(
		nil, nil, requests, nil, nil, decisions, nil, nil, nil,
		nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, nil,
		nativeUploads.UploadFiles{}, enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation})
	resource.ListExportService = lists.NewRendererService()
	return &signatureEnv{
		db: db, router: testpkg.TenantRuntimeMiddleware(t, db)(resource.Router(api.TenantSessionChain)),
		tenantID: tenantID, phaseID: phase.ID, requests: requests,
	}
}

// signaturePNG draws a short stroke, the smallest image the checks accept.
func signaturePNG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 300, 100))
	for x := 20; x < 280; x++ {
		img.Set(x, 50+(x%9), color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func (env *signatureEnv) submission(email, guardianFirstName string, signature *capability.SubmitSignature) enrollmentAPI.SubmitRequest {
	return enrollmentAPI.SubmitRequest{
		TenantID: env.tenantID, PhaseID: env.phaseID,
		GuardianFirstName: guardianFirstName, GuardianLastName: "Beispiel", GuardianEmail: email,
		ConsentFlags: map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": false},
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Lina", LastName: "Beispiel", DateOfBirth: calendar.NewDate(2018, 4, 15), TargetGradeLevel: testpkg.Int16Ptr(1),
		}},
		Signature: signature,
	}
}

func (env *signatureEnv) submitSigned(t *testing.T, email, guardianFirstName string) *enrollmentAPI.SubmitResult {
	t.Helper()
	result, err := env.requests.Submit(testpkg.TenantContext(env.tenantID), env.submission(email, guardianFirstName,
		&capability.SubmitSignature{Method: "drawn", ImagePNG: signaturePNG(t)}))
	require.NoError(t, err)
	return result
}

func (env *signatureEnv) staff(t *testing.T, tenantID int64) string {
	t.Helper()
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Vera", "Verwaltung")
	return testutil.MintTestJWT(t, sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, Username: "verwaltung" + strconv.FormatInt(account.ID, 10),
		Roles: []string{"admin"}, Permissions: managePermissions, TenantID: tenantID,
	})
}

func (env *signatureEnv) do(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

// unsignedRequest stores a request of the school without a signature, the
// way an older submission before the form required one looks. The school
// must be the test's fixture tenant.
func (env *signatureEnv) unsignedRequest(t *testing.T) string {
	t.Helper()
	require.Equal(t, testpkg.Tenant(t), env.tenantID)
	token := "unsigned-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, env.db, env.phaseID, "ohne-unterschrift@example.com", token)
	return token
}

type statusSignatureBody struct {
	Data struct {
		SignatureRequired bool `json:"signature_required"`
		Signature         *struct {
			Method      string `json:"method"`
			SignerName  string `json:"signer_name"`
			IntegrityOK bool   `json:"integrity_ok"`
			ImagePNG    string `json:"image_png"`
			DocumentSHA string `json:"document_sha256"`
		} `json:"signature"`
	} `json:"data"`
}

func decodeStatusSignature(t *testing.T, rec *httptest.ResponseRecorder) statusSignatureBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body statusSignatureBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func requireErrorCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, code, body.Code)
}

func requirePDF(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")), "the proof is a PDF")
}

func TestEnrollmentSignature_RequiredFormRefusesMissingAndInvalidSignatures(t *testing.T) {
	t.Parallel()
	env := setupSignatureTest(t, testpkg.UniqueTestTenantID(t))
	ctx := testpkg.TenantContext(env.tenantID)

	_, err := env.requests.Submit(ctx, env.submission("ohne@example.com", "Ohne", nil))
	var rejection *capability.InvalidInputError
	require.ErrorAs(t, err, &rejection, "a signed form refuses a submission without a signature")
	assert.Equal(t, capability.CodeSignatureRequired, rejection.Code)
	assert.Equal(t, "signature", rejection.Field)

	empty := image.NewNRGBA(image.Rect(0, 0, 300, 100))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, empty))
	_, err = env.requests.Submit(ctx, env.submission("leer@example.com", "Leer",
		&capability.SubmitSignature{Method: "drawn", ImagePNG: base64.StdEncoding.EncodeToString(buf.Bytes())}))
	require.ErrorAs(t, err, &rejection, "an empty pad is no signature")
	assert.Equal(t, capability.CodeSignatureInvalid, rejection.Code)
	require.ErrorIs(t, err, capability.ErrSignatureInvalid)

	_, err = env.requests.Submit(ctx, env.submission("text@example.com", "Text",
		&capability.SubmitSignature{Method: "drawn", ImagePNG: base64.StdEncoding.EncodeToString([]byte("kein Bild"))}))
	require.ErrorAs(t, err, &rejection, "a non-PNG is refused")
	assert.Equal(t, capability.CodeSignatureInvalid, rejection.Code)

	var stored int
	require.NoError(t, env.db.NewSelect().TableExpr("audit.signatures").ColumnExpr("count(*)").
		Where("tenant_id = ?", env.tenantID).Scan(ctx, &stored))
	assert.Zero(t, stored, "a refused submission stores no signature")
}

func TestEnrollmentSignature_StatusLinkAndStaffShowTheSignatureAndItsProof(t *testing.T) {
	t.Parallel()
	env := setupSignatureTest(t, testpkg.UniqueTestTenantID(t))
	signed := env.submitSigned(t, "anna@example.com", "Anna")
	token := signed.Request.StatusToken

	status := decodeStatusSignature(t, env.do(t, http.MethodGet, "/requests/"+token, "", ""))
	assert.True(t, status.Data.SignatureRequired)
	require.NotNil(t, status.Data.Signature)
	assert.Equal(t, "drawn", status.Data.Signature.Method)
	assert.Equal(t, "Anna Beispiel", status.Data.Signature.SignerName)
	assert.True(t, status.Data.Signature.IntegrityOK)
	assert.Len(t, status.Data.Signature.DocumentSHA, 64)
	assert.Empty(t, status.Data.Signature.ImagePNG, "the status page does not carry the image")

	requirePDF(t, env.do(t, http.MethodGet, "/requests/"+token+"/signature-proof", "", ""))

	staff := env.staff(t, env.tenantID)
	detail := decodeStatusSignature(t, env.do(t, http.MethodGet, idPath(signed.Request.ID, "/"), staff, ""))
	require.NotNil(t, detail.Data.Signature)
	assert.NotEmpty(t, detail.Data.Signature.ImagePNG, "the staff detail shows the image")
	assert.Equal(t, status.Data.Signature.DocumentSHA, detail.Data.Signature.DocumentSHA)
	requirePDF(t, env.do(t, http.MethodGet, idPath(signed.Request.ID, "/signature-proof"), staff, ""))
}

func TestEnrollmentSignature_ParentEditSignsAgain(t *testing.T) {
	t.Parallel()
	env := setupSignatureTest(t, testpkg.UniqueTestTenantID(t))
	signed := env.submitSigned(t, "bea@example.com", "Bea")
	token := signed.Request.StatusToken
	before := decodeStatusSignature(t, env.do(t, http.MethodGet, "/requests/"+token, "", ""))
	require.NotNil(t, before.Data.Signature)

	edit := `{"phase_id":` + strconv.FormatInt(env.phaseID, 10) + `,"guardian_first_name":"Bea","guardian_last_name":"Beispiel",` +
		`"guardian_email":"bea@example.com","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},` +
		`"children":[{"first_name":"Lina","last_name":"Beispiel","date_of_birth":"2018-04-15","target_grade_level":1}]`
	requireErrorCode(t, env.do(t, http.MethodPut, "/requests/"+token, "", edit+`}`), http.StatusBadRequest, capability.CodeSignatureRequired)
	requireErrorCode(t, env.do(t, http.MethodPatch, "/requests/"+token, "", `{"guardian_first_name":"Beate"}`),
		http.StatusBadRequest, capability.CodeSignatureRequired)

	rec := env.do(t, http.MethodPut, "/requests/"+token, "", edit+`,"signature":{"method":"typed","image_png":"`+signaturePNG(t)+`","typed_name":"Bea Beispiel"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	after := decodeStatusSignature(t, env.do(t, http.MethodGet, "/requests/"+token, "", ""))
	require.NotNil(t, after.Data.Signature)
	assert.Equal(t, "typed", after.Data.Signature.Method, "the newest signature belongs to the edited version")
	assert.NotEqual(t, before.Data.Signature.DocumentSHA, after.Data.Signature.DocumentSHA, "the edited version has its own checksum")
}

// The proof behind the status link is a security boundary: the token alone
// opens it, for its own request in its own school, and nothing else.
func TestEnrollmentSignature_ProofIsBoundToTheTokensSchoolAndRequest(t *testing.T) {
	t.Parallel()
	schoolA := setupSignatureTest(t, testpkg.Tenant(t))
	schoolB := setupSignatureTest(t, testpkg.UniqueTestTenantID(t))
	signedA := schoolA.submitSigned(t, "a@example.com", "Annette")
	signedB := schoolB.submitSigned(t, "b@example.com", "Bernd")

	// Each token reads its own school's signature, whichever school's host
	// serves the route.
	fromA := decodeStatusSignature(t, schoolB.do(t, http.MethodGet, "/requests/"+signedA.Request.StatusToken, "", ""))
	require.NotNil(t, fromA.Data.Signature)
	assert.Equal(t, "Annette Beispiel", fromA.Data.Signature.SignerName)
	fromB := decodeStatusSignature(t, schoolA.do(t, http.MethodGet, "/requests/"+signedB.Request.StatusToken, "", ""))
	require.NotNil(t, fromB.Data.Signature)
	assert.Equal(t, "Bernd Beispiel", fromB.Data.Signature.SignerName)

	// A manipulated or unknown token opens nothing.
	for _, token := range []string{
		signedA.Request.StatusToken + "x",
		strings.ToUpper(signedA.Request.StatusToken),
		signedA.Request.StatusToken[:len(signedA.Request.StatusToken)-1],
		strconv.FormatInt(signedA.Request.ID, 10),
	} {
		requireErrorCode(t, schoolA.do(t, http.MethodGet, "/requests/"+token+"/signature-proof", "", ""),
			http.StatusNotFound, "enrollment.status_link_invalid")
	}

	// Another request of the same school, unsigned, has no proof and does
	// not show the signature of its neighbour.
	unsigned := schoolA.unsignedRequest(t)
	requireErrorCode(t, schoolA.do(t, http.MethodGet, "/requests/"+unsigned+"/signature-proof", "", ""),
		http.StatusNotFound, "enrollment.signature_not_found")
	neighbour := decodeStatusSignature(t, schoolA.do(t, http.MethodGet, "/requests/"+unsigned, "", ""))
	assert.Nil(t, neighbour.Data.Signature)

	// The staff of school B cannot open school A's request or its proof.
	staffB := schoolB.staff(t, schoolB.tenantID)
	assert.Equal(t, http.StatusNotFound, schoolB.do(t, http.MethodGet, idPath(signedA.Request.ID, "/signature-proof"), staffB, "").Code)
	assert.Equal(t, http.StatusNotFound, schoolB.do(t, http.MethodGet, idPath(signedA.Request.ID, "/"), staffB, "").Code)
	var none *capability.RequestSignature
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), schoolB.db, schoolB.tenantID, func(ctx context.Context) error {
		var err error
		none, err = schoolB.requests.RequestSignature(ctx, signedA.Request.ID)
		return err
	}))
	assert.Nil(t, none, "school B reads no signature of school A")

	// An expired token opens nothing either.
	_, err := schoolA.db.NewUpdate().TableExpr("enrollment.requests").
		Set("status_token_expires = ?", time.Now().Add(-time.Hour)).
		Where("id = ?", signedA.Request.ID).Exec(testpkg.TenantContext(schoolA.tenantID))
	require.NoError(t, err)
	requireErrorCode(t, schoolA.do(t, http.MethodGet, "/requests/"+signedA.Request.StatusToken+"/signature-proof", "", ""),
		http.StatusNotFound, "enrollment.status_link_invalid")
}
