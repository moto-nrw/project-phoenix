package messaging

import (
	"context"
	"log/slog"

	"github.com/uptrace/bun"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// TestApplyService exposes the unexported request-apply paths to the external
// messaging_test package. The apply functions are otherwise package-private;
// an internal test cannot reach the services factory (import cycle), so the
// real sub-services are injected here and the apply logic is exercised through
// these thin wrappers.
type TestApplyService struct{ svc *service }

func NewTestApplyService(
	students userService.StudentService,
	persons userService.PersonService,
	arrival scheduleService.ArrivalScheduleService,
	pickup scheduleService.PickupScheduleService,
	uc userContextService.UserContextService,
	db *bun.DB,
) *TestApplyService {
	return &TestApplyService{svc: &service{
		students:    students,
		persons:     persons,
		arrival:     arrival,
		pickup:      pickup,
		userContext: uc,
		db:          db,
		logger:      slog.Default(),
	}}
}

func (t *TestApplyService) ApplyCareSchedule(ctx context.Context, req *usersModels.ParentMessage) error {
	return applyCareScheduleRequest(ctx, t.svc, req, 0)
}

func (t *TestApplyService) CareScheduleDiff(ctx context.Context, studentID int64, payload map[string]any) ([]RequestDiffEntry, error) {
	// Call the real shared entry point directly (the careScheduleDiff forwarder
	// was removed — it existed only for this test, conventions Rule 5/8).
	return t.svc.careScheduleDiffFrom(ctx, &diffSource{s: t.svc, studentID: studentID}, payload)
}
