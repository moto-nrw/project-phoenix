// Router-level tests for the Personalstamm of the Träger (#4705, ADR 0089,
// ADR 0232): the Personalverwaltung of a released Träger sees the staff of
// all its schools with proposals, links the employments of one person into
// one record with one personnel number per Träger, and the bank values move
// from the schools into the record. A school keeps seeing only its own
// staff; a role without the right gets 403; no Träger reaches another
// Träger's records; every read of personal data is in the access log of
// each school it shows.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditCompose "github.com/moto-ogs/project-phoenix/modules/auditlog/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

type traegerStaffFixture struct {
	db     *bun.DB
	portal *traegerPortalHandler
	router http.Handler
	admin  *workforce.StaffAdmin
	unit   tenant.UnitOfWork
	office traegerOffice
}

func newTraegerStaffFixture(t *testing.T, db *bun.DB) traegerStaffFixture {
	t.Helper()
	unit := testpkg.TenantRuntime(t, db)
	portal, _ := traegerPortalOn(t, db, unit)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	persons, err := owners.NewPeopleDirectory(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	repos := NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	membership := NewSchoolMembershipSuiteRepositories(testpkg.NewDatabase(t, db)).SchoolMembership()
	audits := owners.NewTestAuditPersistence(testpkg.MustDatabase(db), nil)
	admin, err := NewStaffAdmin(testpkg.MustDatabase(db), repos.Staff, repos.Person, membership, StaffAdminDependencies{
		MasterDataAudit: audits.StaffMasterDataChange, PersonnelNumber: audits.PersonnelNumberChange, DataAccessLog: audits.DataAccessLog,
	})
	require.NoError(t, err)
	tenantAudit, err := auditCompose.NewTenantPersistence(db)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	require.NoError(t, mountTraegerStaffRoutes(router, admin, organizations, persons, membership, tenantAudit.DataAccessLog))
	f := traegerStaffFixture{db: db, portal: portal, router: testpkg.SessionVerifier(router), admin: admin, unit: unit}
	f.office = newTraegerOffice(t, db, portal, "Träger Personal", identityaccess.TraegerRolePersonnel)
	return f
}

func (f traegerStaffFixture) release(t *testing.T, organization int64) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO workforce.staff_releases (organization_id, representation, released_by_operator_id) VALUES (?, 'agreement', 1)`, organization)
	require.NoError(t, err)
}

func (f traegerStaffFixture) call(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.router, token, method, path, body)
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	return envelope.Data
}

// setPersonnelNumber gives an employment of a school its number.
func (f traegerStaffFixture) setPersonnelNumber(t *testing.T, schoolID, staffID int64, number string) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.staff_employment_profiles SET personnel_number = ? WHERE tenant_id = ? AND membership_id = ?`, number, schoolID, staffID)
	require.NoError(t, err)
}

// setBirthday gives the person behind an employment a birthday.
func (f traegerStaffFixture) setBirthday(t *testing.T, personID int64, day string) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = ? WHERE id = ?`, day, personID)
	require.NoError(t, err)
}

// setIBAN seals an IBAN into the school's bank row as the school would.
func (f traegerStaffFixture) setIBAN(t *testing.T, schoolID, staffID int64, iban string) {
	t.Helper()
	cipher, err := BankDataCipher(viper.GetString("bank_data_keys"), "")
	require.NoError(t, err)
	sealed, err := cipher.Seal([]byte(iban), []byte("users.staff_financial_data.iban|"+strconv.FormatInt(schoolID, 10)+"|"+strconv.FormatInt(staffID, 10)))
	require.NoError(t, err)
	_, err = f.db.ExecContext(context.Background(), `INSERT INTO users.staff_financial_data (tenant_id, staff_id, iban_sealed, iban_masked) VALUES (?, ?, ?, ?)`,
		schoolID, staffID, sealed, "•••• "+iban[len(iban)-4:])
	require.NoError(t, err)
}

func (f traegerStaffFixture) schoolValue(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.NewRaw(query, args...).Scan(context.Background(), &n))
	return n
}

// accessEntries counts the access log entries of the Träger by school.
func (f traegerStaffFixture) accessEntries(t *testing.T, organization int64, resource string) map[int64]int {
	t.Helper()
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		N        int   `bun:"n"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT tenant_id, COUNT(*) AS n FROM audit.data_access_log
		WHERE resource_type = ? AND actor_role = 'traeger' AND metadata->>'organization_id' = ? GROUP BY tenant_id`,
		resource, strconv.FormatInt(organization, 10)).Scan(context.Background(), &rows))
	entries := map[int64]int{}
	for _, row := range rows {
		entries[row.TenantID] = row.N
	}
	return entries
}

func (f traegerStaffFixture) inSchool(t *testing.T, schoolID int64) context.Context {
	t.Helper()
	return tenant.WithTenantID(tenant.WithUnitOfWork(context.Background(), f.unit), schoolID)
}

// assertStaffRefusal checks status and registered code of a refusal.
func assertStaffRefusal(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	assert.Equal(t, code, refusalOf(t, rec).Code)
}

type staffOverviewBody struct {
	Release *struct {
		Representation string `json:"representation"`
	} `json:"release"`
	Schools []struct {
		ID int64 `json:"id"`
	} `json:"schools"`
	Records     []staffRecordBody `json:"records"`
	Suggestions []struct {
		Reason   string `json:"reason"`
		RecordID *int64 `json:"staff_record_id"`
		Members  []struct {
			SchoolID int64  `json:"school_id"`
			StaffID  int64  `json:"staff_id"`
			LastName string `json:"last_name"`
		} `json:"members"`
	} `json:"suggestions"`
}

type staffRecordBody struct {
	ID              int64   `json:"id"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	PersonnelNumber *string `json:"personnel_number"`
	HasFinancial    bool    `json:"has_financial"`
	Employments     []struct {
		SchoolID int64 `json:"school_id"`
		StaffID  int64 `json:"staff_id"`
		Active   bool  `json:"active"`
	} `json:"employments"`
	Financial struct {
		IBAN *string `json:"iban"`
	} `json:"financial"`
	Changes []struct {
		Kind     string `json:"kind"`
		Field    string `json:"field"`
		Value    string `json:"value"`
		SchoolID *int64 `json:"school_id"`
	} `json:"changes"`
}

// TestTraegerStaffRecordsLinkThePeopleOfAllSchools walks the acceptance of
// #4705 end to end.
func TestTraegerStaffRecordsLinkThePeopleOfAllSchools(t *testing.T) {
	t.Parallel()
	f := newTraegerStaffFixture(t, testpkg.SetupTestDB(t))
	north, south := f.office.schools[0], f.office.schools[1]

	miraNorth, account := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, north, "Mira", "Sommer")
	miraSouth := testpkg.CreateTestStaffForExistingAccountInTenant(t, f.db, south, account.ID, "Mira", "Sommer-Berg")
	jonas := testpkg.CreateTestStaffForTenant(t, f.db, south, "Jonas", "Keller")
	f.setPersonnelNumber(t, north, miraNorth.ID, "70001")
	f.setIBAN(t, south, miraSouth.ID, "DE89370400440532013000")

	t.Run("without the release the Träger sees no person", func(t *testing.T) {
		rec := f.call(t, f.office.token, http.MethodGet, "/traeger/staff-records/", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		overview := decode[staffOverviewBody](t, rec)
		assert.Nil(t, overview.Release)
		assert.Len(t, overview.Schools, 2)
		assert.Empty(t, overview.Suggestions)
		rec = f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/", map[string]any{
			"members": []map[string]int64{{"school_id": north, "staff_id": miraNorth.ID}},
		})
		assertStaffRefusal(t, rec, http.StatusConflict, "workforce.traeger_staff_not_released")
		assert.Empty(t, f.accessEntries(t, f.office.organization, "traeger_staff_records"))
	})

	f.release(t, f.office.organization)
	members := []map[string]int64{{"school_id": north, "staff_id": miraNorth.ID}, {"school_id": south, "staff_id": miraSouth.ID}}

	t.Run("the overview proposes the same account at both schools", func(t *testing.T) {
		rec := f.call(t, f.office.token, http.MethodGet, "/traeger/staff-records/", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		overview := decode[staffOverviewBody](t, rec)
		require.NotNil(t, overview.Release)
		assert.Equal(t, "agreement", overview.Release.Representation)
		require.NotEmpty(t, overview.Suggestions)
		first := overview.Suggestions[0]
		assert.Equal(t, workforce.TraegerStaffMatchAccount, first.Reason)
		require.Len(t, first.Members, 2)
		assert.ElementsMatch(t, []int64{miraNorth.ID, miraSouth.ID}, []int64{first.Members[0].StaffID, first.Members[1].StaffID})
		assert.Equal(t, map[int64]int{north: 1, south: 1}, f.accessEntries(t, f.office.organization, "traeger_staff_records"),
			"each school whose people the overview shows logs the read")
	})

	t.Run("a link needs a choice where the schools disagree", func(t *testing.T) {
		rec := f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/link-preview", map[string]any{"members": members})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var preview struct {
			Fields []struct {
				Field    string `json:"field"`
				Conflict bool   `json:"conflict"`
				Options  []struct {
					Source string `json:"source"`
					Value  string `json:"value"`
				} `json:"options"`
			} `json:"fields"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &struct{ Data any }{Data: &preview}))
		conflicts := map[string]bool{}
		for _, field := range preview.Fields {
			conflicts[field.Field] = field.Conflict
			if field.Field == "iban" {
				require.Len(t, field.Options, 1)
				assert.Equal(t, "•••• 3000", field.Options[0].Value, "the preview shows the mask only")
			}
		}
		assert.True(t, conflicts["last_name"])
		assert.False(t, conflicts["first_name"])
		rec = f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/", map[string]any{"members": members})
		assertStaffRefusal(t, rec, http.StatusBadRequest, "workforce.traeger_staff_link_invalid")
	})

	var record staffRecordBody
	t.Run("the link takes the chosen values and the bank values of the schools", func(t *testing.T) {
		rec := f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/", map[string]any{
			"members": members, "choices": map[string]string{"last_name": strconv.FormatInt(south, 10)},
		})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		record = decode[staffRecordBody](t, rec)
		assert.Equal(t, "Sommer-Berg", record.LastName)
		require.NotNil(t, record.PersonnelNumber)
		assert.Equal(t, "70001", *record.PersonnelNumber)
		assert.True(t, record.HasFinancial)
		require.NotNil(t, record.Financial.IBAN)
		assert.Equal(t, "•••• 3000", *record.Financial.IBAN)
		assert.Len(t, record.Employments, 2)
		notChosen := 0
		for _, change := range record.Changes {
			if change.Kind == workforce.TraegerStaffChangeNotChosen {
				notChosen++
				assert.Equal(t, "last_name", change.Field)
				assert.Equal(t, "Sommer", change.Value)
				require.NotNil(t, change.SchoolID)
				assert.Equal(t, north, *change.SchoolID, "the value not taken keeps its school")
			}
		}
		assert.Equal(t, 1, notChosen)
		assert.Zero(t, f.schoolValue(t, `SELECT COUNT(*) FROM users.staff_financial_data WHERE staff_id IN (?, ?)`, miraNorth.ID, miraSouth.ID),
			"no school keeps a copy of the bank values")
		assert.Equal(t, 1, f.schoolValue(t, `SELECT COUNT(*) FROM users.staff_employment_profiles WHERE tenant_id = ? AND membership_id = ? AND personnel_number = '70001'`, south, miraSouth.ID),
			"every Einsatzort carries the Träger's personnel number")
		rec = f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/", map[string]any{"members": members[:1]})
		assertStaffRefusal(t, rec, http.StatusConflict, "workforce.traeger_staff_already_linked")
	})

	t.Run("the bank values open only after the access log of every Einsatzort", func(t *testing.T) {
		rec := f.call(t, f.office.token, http.MethodPost, fmt.Sprintf("/traeger/staff-records/%d/financial/reveal", record.ID), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		financial := decode[workforce.TraegerStaffFinancial](t, rec)
		require.NotNil(t, financial.IBAN)
		assert.Equal(t, "DE89370400440532013000", *financial.IBAN)
		assert.Equal(t, map[int64]int{north: 1, south: 1}, f.accessEntries(t, f.office.organization, "traeger_staff_financial"))
	})

	t.Run("the school reads that the Träger keeps the values and changes none of them", func(t *testing.T) {
		kept, err := f.admin.StaffKeptByTraeger(f.inSchool(t, south), miraSouth.ID)
		require.NoError(t, err)
		assert.True(t, kept)
		kept, err = f.admin.StaffKeptByTraeger(f.inSchool(t, south), jonas.ID)
		require.NoError(t, err)
		assert.False(t, kept)
		number := "70099"
		_, err = f.admin.UpdatePersonnelNumber(f.inSchool(t, south), miraSouth.ID, &number, jonas.ID, "")
		require.ErrorIs(t, err, workforce.ErrStaffKeptByTraeger)
	})

	t.Run("a school sees only its own staff", func(t *testing.T) {
		var records, foreignLinks int
		school, err := tenant.NewTenantID(north)
		require.NoError(t, err)
		require.NoError(t, tenant.WithinTenant(f.inSchool(t, north), school, func(ctx context.Context) error {
			tx, err := tenant.CurrentTransaction[bun.Tx](ctx)
			if err != nil {
				return err
			}
			if err := tx.NewRaw(`SELECT COUNT(*) FROM workforce.staff_records`).Scan(ctx, &records); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT COUNT(*) FROM workforce.staff_record_links WHERE school_id <> ?`, north).Scan(ctx, &foreignLinks)
		}))
		assert.Zero(t, records, "a school reads no record of its Träger")
		assert.Zero(t, foreignLinks, "a school reads no link of another school")
	})

	t.Run("the personnel number is unique per Träger and reaches every Einsatzort", func(t *testing.T) {
		f.setPersonnelNumber(t, south, jonas.ID, "70002")
		rec := f.call(t, f.office.token, http.MethodPut, fmt.Sprintf("/traeger/staff-records/%d/personnel-number", record.ID), map[string]any{"personnel_number": "70002"})
		assertStaffRefusal(t, rec, http.StatusConflict, "workforce.personnel_number_taken")
		rec = f.call(t, f.office.token, http.MethodPut, fmt.Sprintf("/traeger/staff-records/%d/personnel-number", record.ID), map[string]any{"personnel_number": "70005"})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, 2, f.schoolValue(t, `SELECT COUNT(*) FROM users.staff_employment_profiles WHERE membership_id IN (?, ?) AND personnel_number = '70005'`, miraNorth.ID, miraSouth.ID))
		jonasRecord := f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/", map[string]any{
			"members": []map[string]int64{{"school_id": south, "staff_id": jonas.ID}},
		})
		require.Equal(t, http.StatusCreated, jonasRecord.Code, jonasRecord.Body.String())
		rec = f.call(t, f.office.token, http.MethodPut, fmt.Sprintf("/traeger/staff-records/%d/personnel-number", decode[staffRecordBody](t, jonasRecord).ID), map[string]any{"personnel_number": "70005"})
		assertStaffRefusal(t, rec, http.StatusConflict, "workforce.personnel_number_taken")
	})

	t.Run("no role without the right and no other Träger reaches the records", func(t *testing.T) {
		overviewOnly := newTraegerOffice(t, f.db, f.portal, "Träger Übersicht", identityaccess.TraegerRoleReadOnly)
		rec := f.call(t, overviewOnly.token, http.MethodGet, "/traeger/staff-records/", nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		other := newTraegerOffice(t, f.db, f.portal, "Träger Fremd", identityaccess.TraegerRolePersonnel)
		f.release(t, other.organization)
		rec = f.call(t, other.token, http.MethodGet, fmt.Sprintf("/traeger/staff-records/%d", record.ID), nil)
		assertStaffRefusal(t, rec, http.StatusNotFound, "workforce.traeger_staff_record_not_found")
		rec = f.call(t, other.token, http.MethodGet, "/traeger/staff-records/", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Empty(t, decode[staffOverviewBody](t, rec).Records)
	})

	t.Run("dissolving gives the schools their bank values back", func(t *testing.T) {
		rec := f.call(t, f.office.token, http.MethodDelete, fmt.Sprintf("/traeger/staff-records/%d", record.ID), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, 2, f.schoolValue(t, `SELECT COUNT(*) FROM users.staff_financial_data WHERE staff_id IN (?, ?) AND iban_masked = '•••• 3000'`, miraNorth.ID, miraSouth.ID))
		masked, err := f.admin.StaffFinancialMasked(f.inSchool(t, north), miraNorth.ID, account.ID, "admin")
		require.NoError(t, err)
		assert.False(t, masked.KeptByTraeger)
		require.NotNil(t, masked.IBANMasked)
		rec = f.call(t, f.office.token, http.MethodGet, fmt.Sprintf("/traeger/staff-records/%d", record.ID), nil)
		assertStaffRefusal(t, rec, http.StatusNotFound, "workforce.traeger_staff_record_not_found")
	})
}

// TestTraegerStaffOverviewQueryBudget pins the statements of the overview:
// flat in the number of people and records at a fixed set of schools.
func TestTraegerStaffOverviewQueryBudget(t *testing.T) {
	t.Parallel()
	f := newTraegerStaffFixture(t, testpkg.SetupTestDB(t))
	f.release(t, f.office.organization)
	read := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/staff-records/", nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.router, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	add := func(n int) {
		for i := range n {
			for _, school := range f.office.schools {
				staff := testpkg.CreateTestStaffForTenant(t, f.db, school, "Kraft", fmt.Sprintf("Nummer %d", i))
				if i == 0 {
					rec := f.call(t, f.office.token, http.MethodPost, "/traeger/staff-records/", map[string]any{
						"members": []map[string]int64{{"school_id": school, "staff_id": staff.ID}},
					})
					require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
				}
			}
		}
	}
	add(1)
	few := read()
	add(3)
	assert.Len(t, read().Statements, len(few.Statements), "the overview loads set-based")
	querybudgets.AssertQueryBudget(t, "api.traeger.staff_records", few)
}
