// Router-level tests for the master data of a Träger (#4653, ADR 0112): the
// Träger-Büro keeps its logo, its contact person and its bank accounts; a
// role without the right "Einstellungen" reads but cannot change them; no
// Träger reaches the master data of another; every read and change is
// logged.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type masterDataBankAccount struct {
	ID            int64  `json:"id,omitempty"`
	Label         string `json:"label"`
	AccountHolder string `json:"account_holder"`
	IBAN          string `json:"iban"`
	BIC           string `json:"bic"`
	BankName      string `json:"bank_name"`
	IsDefault     bool   `json:"is_default"`
}

type masterDataContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type masterDataLogo struct {
	ContentType string `json:"content_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	SizeBytes   int    `json:"size_bytes"`
}

type masterDataProfile struct {
	Contact      masterDataContact       `json:"contact"`
	Logo         *masterDataLogo         `json:"logo"`
	BankAccounts []masterDataBankAccount `json:"bank_accounts"`
}

// newMasterDataPortal mounts /traeger as the serving root does.
func newMasterDataPortal(t *testing.T) (*bun.DB, *traegerPortalHandler) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	portal, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	return db, portal
}

func newMasterDataOffice(t *testing.T, db *bun.DB, router *traegerPortalHandler, name, role string) traegerOffice {
	t.Helper()
	office := newTraegerOffice(t, db, router, name, role)
	testpkg.OwnTraegerMasterData(t, db, office.organization)
	return office
}

func masterDataProfileOf(t *testing.T, router http.Handler, office traegerOffice) masterDataProfile {
	t.Helper()
	rec := traegerJSON(t, router, office.token, http.MethodGet, "/traeger/master-data/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[masterDataProfile](t, rec.Body.Bytes())
}

func bankAccountPath(id int64) string {
	return "/traeger/master-data/bank-accounts/" + strconv.FormatInt(id, 10)
}

func refusalOf(t *testing.T, rec *httptest.ResponseRecorder) holidayErrorBody {
	t.Helper()
	var refusal holidayErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal), rec.Body.String())
	return refusal
}

func pngLogo(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		img.Set(x, 0, color.RGBA{R: 31, G: 78, B: 121, A: 255})
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func uploadLogo(t *testing.T, router http.Handler, token, field, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPut, "/traeger/master-data/logo", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func masterDataLog(t *testing.T, db *bun.DB, organizationID int64) []string {
	t.Helper()
	var entries []string
	require.NoError(t, db.NewRaw(`SELECT subject || ':' || action FROM platform.traeger_master_data_audit
		WHERE organization_id = ? AND actor_account_id IS NOT NULL ORDER BY id`, organizationID).Scan(context.Background(), &entries))
	return entries
}

const (
	traegerTestIBAN  = "DE89370400440532013000"
	traegerOtherIBAN = "DE02120300000000202051"
)

func TestTraegerBankAccountsKeepOneDefault(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newMasterDataOffice(t, db, router, "Träger Bissendorf Stammdaten", identityaccess.TraegerRoleAdministration)

	profile := masterDataProfileOf(t, router, office)
	assert.Empty(t, profile.BankAccounts)
	assert.Nil(t, profile.Logo)
	assert.Equal(t, masterDataContact{}, profile.Contact)

	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: " Elternbeiträge ", AccountHolder: "Träger Bissendorf e.V.", IBAN: "de89 3704 0044 0532 0130 00", BIC: "cobadeffxxx", BankName: "Commerzbank",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	first := decodeSchoolPortalData[masterDataBankAccount](t, rec.Body.Bytes())
	assert.Equal(t, masterDataBankAccount{ID: first.ID, Label: "Elternbeiträge", AccountHolder: "Träger Bissendorf e.V.", IBAN: traegerTestIBAN,
		BIC: "COBADEFFXXX", BankName: "Commerzbank", IsDefault: true}, first, "the first bank account is the default; IBAN and BIC are compacted")

	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "Ferien", AccountHolder: "Träger Bissendorf e.V.", IBAN: traegerOtherIBAN,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	second := decodeSchoolPortalData[masterDataBankAccount](t, rec.Body.Bytes())
	assert.False(t, second.IsDefault)

	second.IsDefault = true
	rec = traegerJSON(t, router, office.token, http.MethodPut, bankAccountPath(second.ID), second)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	profile = masterDataProfileOf(t, router, office)
	require.Len(t, profile.BankAccounts, 2)
	assert.Equal(t, second.ID, profile.BankAccounts[0].ID, "the default comes first")
	assert.True(t, profile.BankAccounts[0].IsDefault)
	assert.False(t, profile.BankAccounts[1].IsDefault, "the old default gave up the flag")

	// Clearing the flag of the default keeps it: the Träger chooses
	// another one instead.
	second.IsDefault = false
	second.BankName = "Sparkasse"
	rec = traegerJSON(t, router, office.token, http.MethodPut, bankAccountPath(second.ID), second)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, decodeSchoolPortalData[masterDataBankAccount](t, rec.Body.Bytes()).IsDefault)

	rec = traegerJSON(t, router, office.token, http.MethodDelete, bankAccountPath(second.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	profile = masterDataProfileOf(t, router, office)
	require.Len(t, profile.BankAccounts, 1)
	assert.Equal(t, first.ID, profile.BankAccounts[0].ID)
	assert.True(t, profile.BankAccounts[0].IsDefault, "deleting the default makes the remaining one the default")

	assert.Equal(t, []string{
		"master_data:read", "bank_account:create", "bank_account:create", "bank_account:update", "bank_account:update", "master_data:read",
		"bank_account:update", "bank_account:delete", "bank_account:update", "master_data:read",
	}, masterDataLog(t, db, office.organization), "every read and change is logged with the acting account, the old default's demotion too")
	var demotion struct {
		SubjectID int64           `bun:"subject_id"`
		Old       json.RawMessage `bun:"old_value"`
		New       json.RawMessage `bun:"new_value"`
	}
	require.NoError(t, db.NewRaw(`SELECT subject_id, old_value, new_value FROM platform.traeger_master_data_audit
		WHERE organization_id = ? AND subject = 'bank_account' AND action = 'update' ORDER BY id LIMIT 1`, office.organization).
		Scan(context.Background(), &demotion))
	assert.Equal(t, first.ID, demotion.SubjectID, "the first update takes the flag from the old default")
	assert.Contains(t, string(demotion.Old), `"is_default": true`)
	assert.Contains(t, string(demotion.New), `"is_default": false`)
}

func TestTraegerMasterDataRefusesInvalidInput(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newMasterDataOffice(t, db, router, "Träger Prüfung Stammdaten", identityaccess.TraegerRoleAdministration)

	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "", AccountHolder: "Träger e.V.", IBAN: "DE89370400440532013001", BIC: "COBA",
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	refusal := refusalOf(t, rec)
	assert.Equal(t, "traeger.master_data_invalid", refusal.Code)
	fields := []string{}
	for _, field := range refusal.Errors {
		fields = append(fields, field.Field)
	}
	assert.ElementsMatch(t, []string{"label", "iban", "bic"}, fields, "a wrong check digit fails the IBAN")

	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "Elternbeiträge", AccountHolder: "Träger e.V.", IBAN: traegerTestIBAN,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, office.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "elternbeiträge", AccountHolder: "Träger e.V.", IBAN: traegerOtherIBAN,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	refusal = refusalOf(t, rec)
	assert.Equal(t, "traeger.bank_account_label_taken", refusal.Code)
	require.Len(t, refusal.Errors, 1)
	assert.Equal(t, "label", refusal.Errors[0].Field)

	rec = traegerJSON(t, router, office.token, http.MethodPut, "/traeger/master-data/contact", masterDataContact{Name: "Frau Beispiel", Phone: "Büro", Email: "keine-adresse"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	refusal = refusalOf(t, rec)
	assert.Equal(t, "traeger.master_data_invalid", refusal.Code)
	fields = fields[:0]
	for _, field := range refusal.Errors {
		fields = append(fields, field.Field)
	}
	assert.ElementsMatch(t, []string{"phone", "email"}, fields)

	rec = traegerJSON(t, router, office.token, http.MethodPut, "/traeger/master-data/contact", masterDataContact{
		Name: " Frau Beispiel ", Phone: "+49 5402 123-45", Email: "Buero@Traeger-Beispiel.de",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, masterDataContact{Name: "Frau Beispiel", Phone: "+49 5402 123-45", Email: "buero@traeger-beispiel.de"},
		masterDataProfileOf(t, router, office).Contact)
}

func TestTraegerLogoUploadChecksTypeSizeAndContent(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newMasterDataOffice(t, db, router, "Träger Logo", identityaccess.TraegerRoleAdministration)
	logo := pngLogo(t, 40, 20)

	rec := uploadLogo(t, router, office.token, "logo", "logo.png", logo)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored := decodeSchoolPortalData[masterDataLogo](t, rec.Body.Bytes())
	assert.Equal(t, masterDataLogo{ContentType: "image/png", Width: 40, Height: 20, SizeBytes: len(logo)}, stored)
	profile := masterDataProfileOf(t, router, office)
	require.NotNil(t, profile.Logo)
	assert.Equal(t, stored, *profile.Logo)

	req := httptest.NewRequest(http.MethodGet, "/traeger/master-data/logo", nil)
	req.Header.Set("Authorization", "Bearer "+office.token)
	served := httptest.NewRecorder()
	router.ServeHTTP(served, req)
	require.Equal(t, http.StatusOK, served.Code, served.Body.String())
	assert.Equal(t, "image/png", served.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", served.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, logo, served.Body.Bytes())

	refusals := []struct {
		name, field, filename string
		data                  []byte
		code                  string
	}{
		{"text renamed to png", "logo", "logo.png", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"), "traeger.logo_invalid"},
		{"png named gif", "logo", "logo.gif", logo, "traeger.logo_invalid"},
		{"truncated png", "logo", "logo.png", logo[:len(logo)/2], "traeger.logo_invalid"},
		{"too many pixels", "logo", "logo.png", pngLogo(t, 2001, 10), "traeger.logo_invalid"},
		{"over one megabyte", "logo", "logo.png", append(bytes.Clone(logo), make([]byte, 1<<20)...), "traeger.logo_too_large"},
		{"wrong field", "file", "logo.png", logo, "traeger.logo_invalid"},
	}
	for _, refused := range refusals {
		rec := uploadLogo(t, router, office.token, refused.field, refused.filename, refused.data)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", refused.name, rec.Body.String())
		assert.Equal(t, refused.code, refusalOf(t, rec).Code, refused.name)
	}
	profile = masterDataProfileOf(t, router, office)
	require.NotNil(t, profile.Logo)
	assert.Equal(t, stored, *profile.Logo, "a refused upload keeps the stored logo")

	rec = traegerJSON(t, router, office.token, http.MethodDelete, "/traeger/master-data/logo", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, office.token, http.MethodGet, "/traeger/master-data/logo", nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "traeger.logo_not_found", refusalOf(t, rec).Code)
	assert.Nil(t, masterDataProfileOf(t, router, office).Logo)
}

func TestTraegerMasterDataNeedsTheSettingsRight(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	admin := newMasterDataOffice(t, db, router, "Träger Rechte Stammdaten", identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, router, admin.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "Elternbeiträge", AccountHolder: "Träger e.V.", IBAN: traegerTestIBAN,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	account := decodeSchoolPortalData[masterDataBankAccount](t, rec.Body.Bytes())

	for _, role := range []string{identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRoleBilling, identityaccess.TraegerRolePersonnel} {
		reader := newMasterDataOffice(t, db, router, "Träger Rolle "+role, role)
		profile := masterDataProfileOf(t, router, reader)
		assert.Empty(t, profile.BankAccounts, "%s reads only its own Träger", role)
		writes := []*httptest.ResponseRecorder{
			traegerJSON(t, router, reader.token, http.MethodPut, "/traeger/master-data/contact", masterDataContact{Name: "Herr Beispiel"}),
			traegerJSON(t, router, reader.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{Label: "Neu", AccountHolder: "X", IBAN: traegerTestIBAN}),
			traegerJSON(t, router, reader.token, http.MethodPut, bankAccountPath(account.ID), account),
			traegerJSON(t, router, reader.token, http.MethodDelete, bankAccountPath(account.ID), nil),
			traegerJSON(t, router, reader.token, http.MethodDelete, "/traeger/master-data/logo", nil),
			uploadLogo(t, router, reader.token, "logo", "logo.png", pngLogo(t, 4, 4)),
		}
		for i, write := range writes {
			assert.Equal(t, http.StatusForbidden, write.Code, "%s write %d: %s", role, i, write.Body.String())
		}
	}
	assert.Len(t, masterDataProfileOf(t, router, admin).BankAccounts, 1, "no refused write reached the Träger")
}

func TestTraegerMasterDataStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	own := newMasterDataOffice(t, db, router, "Träger Hardt Stammdaten", identityaccess.TraegerRoleAdministration)
	other := newMasterDataOffice(t, db, router, "Träger Willich Stammdaten", identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, router, own.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "Elternbeiträge", AccountHolder: "Träger Hardt e.V.", IBAN: traegerTestIBAN,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	account := decodeSchoolPortalData[masterDataBankAccount](t, rec.Body.Bytes())
	require.Equal(t, http.StatusOK, uploadLogo(t, router, own.token, "logo", "logo.png", pngLogo(t, 8, 8)).Code)
	require.Equal(t, http.StatusOK, traegerJSON(t, router, own.token, http.MethodPut, "/traeger/master-data/contact", masterDataContact{Name: "Frau Hardt"}).Code)

	profile := masterDataProfileOf(t, router, other)
	assert.Empty(t, profile.BankAccounts)
	assert.Nil(t, profile.Logo)
	assert.Equal(t, masterDataContact{}, profile.Contact)
	rec = traegerJSON(t, router, other.token, http.MethodGet, "/traeger/master-data/logo", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	account.Label = "Übernommen"
	rec = traegerJSON(t, router, other.token, http.MethodPut, bankAccountPath(account.ID), account)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "traeger.bank_account_not_found", refusalOf(t, rec).Code)
	rec = traegerJSON(t, router, other.token, http.MethodDelete, bankAccountPath(account.ID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, router, other.token, http.MethodDelete, "/traeger/master-data/logo", nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	var otherRows int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.traeger_profiles WHERE organization_id = ?`, other.organization).
		Scan(context.Background(), &otherRows))
	assert.Zero(t, otherRows, "a refused change writes nothing, although the route commits its 404")

	profile = masterDataProfileOf(t, router, own)
	require.Len(t, profile.BankAccounts, 1)
	assert.Equal(t, "Elternbeiträge", profile.BankAccounts[0].Label, "the foreign Träger changed nothing")
	assert.Equal(t, "Frau Hardt", profile.Contact.Name)
}

func TestTraegerMasterDataQueryBudget(t *testing.T) {
	t.Parallel()
	db, router := newMasterDataPortal(t)
	office := newMasterDataOffice(t, db, router, "Träger Budget Stammdaten", identityaccess.TraegerRoleAdministration)
	addAccount := func(label, iban string) {
		rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
			Label: label, AccountHolder: "Träger e.V.", IBAN: iban,
		})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	addAccount("Elternbeiträge", traegerTestIBAN)
	read := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/master-data/", nil)
		req.Header.Set("Authorization", "Bearer "+office.token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	one := read()
	addAccount("Ferien", traegerOtherIBAN)
	addAccount("Mensa", "GB82WEST12345698765432")
	assert.Len(t, read(), len(one), "the master data reads the same statements for any number of bank accounts")
	testpkg.AssertQueryBudget(t, "api.traeger.master_data", one)
}
