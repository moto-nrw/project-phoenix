package students_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db          *bun.DB
	services    *services.Factory
	resource    *studentsAPI.Resource
	broadcaster *recordingBroadcaster
}

type recordingBroadcaster struct {
	events []realtime.Event
}

func (b *recordingBroadcaster) BroadcastToGroup(_ int64, _ string, event realtime.Event) error {
	b.events = append(b.events, event)
	return nil
}

func (b *recordingBroadcaster) BroadcastToTenant(_ int64, event realtime.Event) error {
	b.events = append(b.events, event)
	return nil
}

func (b *recordingBroadcaster) BroadcastToAll(event realtime.Event) error {
	b.events = append(b.events, event)
	return nil
}

func (b *recordingBroadcaster) BroadcastToTenant(_ int64, event realtime.Event) error {
	b.events = append(b.events, event)
	return nil
}

// setupTestContext initializes the test environment.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	svc, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	broadcaster := &recordingBroadcaster{}

	resource := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:          svc.Users,
		StudentRepo:            repoFactory.Student,
		EducationService:       svc.Education,
		UserContextService:     svc.UserContext,
		ActiveService:          svc.Active,
		IoTService:             svc.IoT,
		PrivacyConsentRepo:     repoFactory.PrivacyConsent,
		PickupScheduleService:  svc.PickupSchedule,
		ArrivalScheduleService: svc.ArrivalSchedule,
		SchoolRepo:             repoFactory.School,
		SettingsService:        svc.Settings,
		AttendanceRepo:         repoFactory.Attendance,
		StudentStatusDayRepo:   repoFactory.StudentStatusDay,
		VisitRepo:              repoFactory.ActiveVisit,
		DataAccessLogRepo:      repoFactory.DataAccessLog,
		Broadcaster:            broadcaster,
		Logger:                 slog.Default(),
		DB:                     db,
	})

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:          db,
		services:    svc,
		resource:    resource,
		broadcaster: broadcaster,
	}
}

// setupRouter creates a Chi router with the given handler.
func setupRouter(handler http.HandlerFunc, urlParam string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	if urlParam != "" {
		router.Get(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Put(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Delete(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Post(fmt.Sprintf("/{%s}", urlParam), handler)
	} else {
		router.Get("/", handler)
		router.Post("/", handler)
	}
	return router
}

// executeWithAuth executes a request with JWT claims and permissions.
func executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	if claims.TenantID != 0 {
		ctx = tenant.WithTenantID(ctx, claims.TenantID)
	}
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}
