package postgres

import (
	context "context"
	errors "errors"
	fmt "fmt"
	io "io"
	time "time"

	organizationtenancy "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type DemoQueueEngine struct{ Store *DemoStateStore }

func (e DemoQueueEngine) RequeueDeferredDemoSchoolOrders(ctx context.Context) error {
	return e.Store.RequeueDeferred(ctx, "")
}
func (e DemoQueueEngine) RequeueDeferredDemoSchoolOrder(ctx context.Context, slug string) error {
	return e.Store.RequeueDeferred(ctx, slug)
}
func (e DemoQueueEngine) ReleaseDemoSchoolOrders(ctx context.Context) error {
	return e.Store.ReleaseClaims(ctx)
}
func (e DemoQueueEngine) ClaimDemoSchoolOrder(ctx context.Context) (*organizationtenancy.DemoSchoolOrder, error) {
	order, err := e.Store.Claim(ctx)
	if err != nil || order == nil {
		return nil, err
	}
	return &organizationtenancy.DemoSchoolOrder{
		Slug: order.Name, SchoolName: order.SchoolName, FirstName: order.FirstName, LastName: order.LastName,
		Attempts: order.Attempts, Seeded: order.Seeded,
	}, nil
}
func (e DemoQueueEngine) FinishDemoSchoolOrder(ctx context.Context, slug string, visitorAccountID, visitorParentAccountID int64) error {
	return e.Store.Finish(ctx, slug, visitorAccountID, visitorParentAccountID)
}
func (e DemoQueueEngine) FailDemoSchoolOrder(ctx context.Context, slug string, maxAttempts int) (bool, error) {
	return e.Store.Fail(ctx, slug, maxAttempts)
}
func (e DemoQueueEngine) ReadyDemoSchools(ctx context.Context) ([]string, error) {
	return e.Store.ReadyNames(ctx)
}
func (e DemoQueueEngine) ActiveDemoSchools(ctx context.Context, since time.Time) ([]string, error) {
	return e.Store.ActiveNames(ctx, since)
}
func (e DemoQueueEngine) ReturnDemoSchoolOrder(ctx context.Context, slug string) error {
	return e.Store.Return(ctx, slug)
}
func (e DemoQueueEngine) RetireDemoSchools(ctx context.Context, slugs []string) (int, int, error) {
	return e.Store.RetireMany(ctx, slugs)
}

type DemoOrderEngine struct {
	Store     *DemoOrderStore
	Random    io.Reader
	MaxActive int
}

const DemoSlugAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

func (e DemoOrderEngine) OrderDemoSchool(ctx context.Context, schoolName, firstName, lastName string) (string, error) {
	suffix := make([]byte, 6)
	if _, err := io.ReadFull(e.Random, suffix); err != nil {
		return "", fmt.Errorf("demo school slug: %w", err)
	}
	for i, b := range suffix {
		suffix[i] = DemoSlugAlphabet[int(b)%len(DemoSlugAlphabet)]
	}
	slug := organizationtenancy.DemoSchoolSlug(schoolName, string(suffix))
	err := e.Store.Enqueue(ctx, slug, schoolName, firstName, lastName, e.MaxActive)
	if errors.Is(err, ErrDemoCapacityReached) {
		return "", organizationtenancy.ErrDemoCapacityReached
	}
	if err != nil {
		return "", err
	}
	return slug, nil
}
func (e DemoOrderEngine) DemoSchoolProgress(ctx context.Context, slug string) (*organizationtenancy.DemoSchoolProgress, error) {
	progress, err := e.Store.Progress(ctx, slug)
	if err != nil || progress == nil {
		return nil, err
	}
	return &organizationtenancy.DemoSchoolProgress{
		Status: progress.Status, SchoolID: progress.TenantID, VisitorAccountID: progress.VisitorAccountID,
		VisitorParentAccountID: progress.VisitorParentAccountID,
	}, nil
}
func (e DemoOrderEngine) MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error {
	return e.Store.MarkUsed(ctx, slug, usedAt)
}
func (e DemoOrderEngine) RetireDemoSchool(ctx context.Context, slug string) error {
	return e.Store.Retire(ctx, slug)
}
