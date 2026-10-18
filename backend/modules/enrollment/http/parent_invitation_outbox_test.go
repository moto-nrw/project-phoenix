package enrollmenthttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestParentInvitationOutboxIsAtomicWithApproval(t *testing.T) {
	t.Parallel()
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprintf("manual=%v", manual), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
			database := testpkg.NewDatabase(t, db)
			repos, err := api.NewEnrollmentTestRepositories(database, api.NewTestAuditStore(database))
			require.NoError(t, err)
			careRows := api.NewEnrollmentCarePlanRows(database)
			schoolRows := api.NewOrganizationTestRows(database)
			timetableRows := api.NewEnrollmentTimetableRows(database)
			_, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Parent", "Reviewer")
			phase := createOfferingGuardPhase(t, repos, ctx, capability.PhaseCareOfferingSelectionOptional)
			schema, err := NewTestFormSchemas(repos.Enrollment()).CreateSchema(ctx, "Atomic parent invitation", []capability.FormField{
				{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
			}, reviewer.ID)
			require.NoError(t, err)
			_, err = db.NewRaw(`UPDATE enrollment.phases SET form_schema_id = ? WHERE id = ?`, schema.ID, phase.ID).Exec(ctx)
			require.NoError(t, err)
			request, child := createOfferingGuardChild(t, repos, ctx, phase.ID)
			guardianAccess, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
			require.NoError(t, err)
			students, err := api.NewPeopleDirectory(database)
			require.NoError(t, err)
			bookings := newBookingMaterialization(t, db, api.WithBookingCatalog(newCareOfferingCatalog(t, db).Catalog), api.WithBookingSettings(offeringGuardSettings(false))).Bookings
			decisions := newParentInvitationDecisions(repos, careRows, timetableRows, guardianAccess, students, bookings)
			decision := enrollmentAPI.NewDecisionService(enrollmentCompose.PublicDecisions(decisions))
			requests := enrollmentAPI.NewRequestService(api.NewEnrollmentIntake(api.EnrollmentIntakeSources{
				Requests: repos.Enrollment(), Children: repos.Enrollment(), Guardians: repos.Enrollment(),
				CareOfferingRepo: enrollmentCompose.NewCareOfferingRecords(careRows.CarePlan), Catalog: repos.Enrollment(),
				SchoolRepo: capabilitySchools{schools: schoolRows.School}, RateLimitRepo: repos.Enrollment(),
				LateInviteRepo: repos.Enrollment(), OutboxEnqueuer: discardingOutbox{}, Settings: stubTakeoverSettings{},
				ManualDecider: decisions, FrontendURL: "http://localhost:3000", ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
			}))
			delivery, err := api.NewDeliveryTestModule(database, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			invitations, err := api.NewGuardianInvitationServiceForTests(database, testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
				Outbox: api.OutboxEnqueuer{Runtime: delivery.Runtime, Outbox: delivery.EmailOutbox},
			})
			require.NoError(t, err)
			resource := enrollmentAPI.NewResource(nil, nil, requests, nil, nil, decision, nil, nil, nil, nil,
				enrollmentAPI.GuardianInvitationRuntime{Create: func(inviteCtx context.Context, profileID, actorID int64) error {
					_, err := invitations.CreateGuardianInvitation(inviteCtx, identityaccess.GuardianInvitationRequest{GuardianProfileID: profileID, CreatedBy: actorID})
					return err
				}}, nil, nil, nativeUploads.UploadFiles{}, enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation})
			router := testpkg.TenantRuntimeMiddleware(t, db)(resource.Router(api.TenantSessionChain))
			path, payload, wantStatus := fmt.Sprintf("/admin/requests/%d/children/%d/decide", request.ID, child.ID), []byte(`{"status":"approved"}`), http.StatusOK
			if manual {
				grade := int16(1)
				payload, err = json.Marshal(enrollmentAPI.AdminManualApprovedEnrollmentRequest{
					SubmitEnrollmentRequest:  enrollmentAPI.SubmitEnrollmentRequest{GuardianFirstName: "Atomic", GuardianLastName: "Parent", GuardianEmail: fmt.Sprintf("atomic-manual-%d@example.test", testpkg.Tenant(t)), Children: []enrollmentAPI.SubmitChildRequest{{FirstName: "Atomic", LastName: "Child", DateOfBirth: "2019-03-01", TargetGradeLevel: &grade}}},
					ExternalConsentConfirmed: true, Reason: "Anmeldung auf Papier", SendNotification: true,
				})
				require.NoError(t, err)
				path, wantStatus = fmt.Sprintf("/phases/%d/manual-approved-enrollments", phase.ID), http.StatusCreated
			}
			count := func(queryCtx context.Context, source bun.IDB, want int) {
				t.Helper()
				n, err := source.NewSelect().TableExpr("platform.email_outbox").Where("tenant_id = ? AND kind = ?", testpkg.Tenant(t), identityaccess.MailKindGuardianInvitation).Count(queryCtx)
				require.NoError(t, err)
				require.EqualValues(t, want, n)
			}
			for _, commit := range []bool{false, true} {
				err := tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload)).WithContext(txCtx)
					req.Header.Set("Authorization", "Bearer "+mintOfferingGuardReviewerToken(t, reviewer.ID, reviewer.Email))
					req.Header.Set("Content-Type", "application/json")
					rec := httptest.NewRecorder()
					router.ServeHTTP(rec, req)
					require.Equal(t, wantStatus, rec.Code, rec.Body.String())
					count(txCtx, tx, 1)
					if !commit {
						return errors.New("rollback approval")
					}
					return nil
				})
				if commit {
					require.NoError(t, err)
					count(ctx, db, 1)
				} else {
					require.ErrorContains(t, err, "rollback approval")
					count(ctx, db, 0)
				}
			}
		})
	}
}

func newParentInvitationDecisions(repos api.EnrollmentTestRepositories, reposCare api.TimetableCarePlanRows, reposTimetable api.TimetableTestRepositories, guardianAccess TestDecisionGuardianAccess, studentEnrollment api.EnrollmentDecisionPeople, bookings TestDecisionBookings) *TestDecisions {
	return api.NewEnrollmentDecisions(enrollmentCompose.DecisionDependencies{
		Requests:       repos.Enrollment(),
		Children:       repos.Enrollment(),
		Guardians:      repos.Enrollment(),
		LateInvites:    repos.Enrollment(),
		Phases:         repos.Enrollment(),
		Schemas:        repos.Enrollment(),
		Offerings:      enrollmentCompose.NewCareOfferingRecords(reposCare.CarePlan),
		Notifications:  NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: parentInvitationSettings()}, discardingOutbox{}, nil),
		Bookings:       bookings,
		GuardianAccess: guardianAccess,
		People: api.EnrollmentDecisionPeopleDirectory(
			repos.Person, repos.Staff, repos.Student, repos.StudentGuardian, repos.GuardianProfile, repos.GuardianPhoneNumber, studentEnrollment,
		),
		StudentEnrollment: studentEnrollment,
		PickupSchedules:   reposTimetable.EnrollmentPickupSchedules(),
		ArrivalSchedules:  reposTimetable.EnrollmentArrivalSchedules(),
		ParentsURL:        api.EnrollmentDecisionParentsURL("http://parents.localhost:3000", ""),
		Settings:          api.EnrollmentDecisionSettingValues(parentInvitationSettings()),
		Logger:            slog.Default(),
	})
}

func parentInvitationSettings() *settingsDouble {
	settings := offeringGuardSettings(false)
	settings.ResolveBoolFn = func(_ context.Context, key string) (bool, error) {
		return key != settingKeyCareOfferingsEnabled, nil
	}
	return settings
}
