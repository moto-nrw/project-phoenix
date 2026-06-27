package students_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	userService "github.com/moto-nrw/project-phoenix/services/users"
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

func (b *recordingBroadcaster) BroadcastParentMessage(_, _ int64, _ realtime.Event) error { return nil }

func (b *recordingBroadcaster) BroadcastToTenant(_ int64, event realtime.Event) error {
	b.events = append(b.events, event)
	return nil
}

func (b *recordingBroadcaster) BroadcastToAll(event realtime.Event) error {
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

	studentPhotos := userService.NewStudentPhotoService(userService.StudentPhotoServiceDependencies{
		StudentRepo: repoFactory.Student,
		Settings:    svc.Settings,
		UserContext: svc.UserContext,
		Broadcaster: broadcaster,
		Unlinker:    studentsAPI.NewPhotoUnlinker(slog.Default()),
		DB:          db,
		Logger:      slog.Default(),
	})

	resource := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:           svc.Users,
		GuardianService:         svc.Guardian,
		StudentService:          userService.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent),
		EducationService:        svc.Education,
		UserContextService:      svc.UserContext,
		ActiveService:           svc.Active,
		IoTService:              svc.IoT,
		PickupScheduleService:   svc.PickupSchedule,
		ArrivalScheduleService:  svc.ArrivalSchedule,
		SchoolService:           svc.Schools,
		SettingsService:         svc.Settings,
		StudentHistoryService:   activeSvc.NewStudentHistoryService(repoFactory.Attendance, repoFactory.ActiveVisit, repoFactory.DataAccessLog),
		InstanceService:         svc.Instance,
		StudentStatusDayService: activeSvc.NewStudentStatusDayService(repoFactory.StudentStatusDay),
		Broadcaster:             broadcaster,
		StudentPhotos:           studentPhotos,
		Logger:                  slog.Default(),
		DB:                      db,
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
