package peopledirectory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// stubProvider answers every guardian operation with canned values and
// records what it was asked.
type stubProvider struct {
	peopledirectory.GuardianProvider
	found    peopledirectory.Guardian
	err      error
	calls    []string
	lastText string
}

func (p *stubProvider) FindGuardian(_ context.Context, id int64) (peopledirectory.Guardian, error) {
	p.calls = append(p.calls, "find")
	return p.found, p.err
}

func (p *stubProvider) SearchGuardians(_ context.Context, text string, _ int) ([]peopledirectory.GuardianMatch, error) {
	p.calls = append(p.calls, "search")
	p.lastText = text
	return nil, p.err
}

func (p *stubProvider) DeleteGuardian(context.Context, peopledirectory.GuardianDelete) error {
	p.calls = append(p.calls, "delete")
	return p.err
}

func TestGuardianCallsFailWhileNoProviderIsBound(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)

	_, err := module.FindGuardian(context.Background(), 7)
	if !errors.Is(err, peopledirectory.ErrGuardianProviderUnbound) {
		t.Fatalf("expected the unbound sentinel, got %v", err)
	}
	if len(engine.observed) != 1 || engine.observed[0] != "find_guardian:unbound" {
		t.Fatalf("the failed call must still be observed, got %v", engine.observed)
	}
	if err := module.UpdateGuardian(context.Background(), 7, peopledirectory.GuardianInput{}); !errors.Is(err, peopledirectory.ErrGuardianProviderUnbound) {
		t.Fatalf("commands report the same sentinel, got %v", err)
	}
}

func TestGuardianQueriesValidateBeforeDelegating(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	provider := &stubProvider{}
	module.BindGuardianProvider(provider)
	ctx := context.Background()

	if _, err := module.FindGuardian(ctx, 0); !errors.Is(err, peopledirectory.ErrInvalidGuardian) {
		t.Fatalf("a zero id is invalid input, got %v", err)
	}
	if _, err := module.ListGuardians(ctx, -1, 10); !errors.Is(err, peopledirectory.ErrInvalidGuardian) {
		t.Fatalf("a negative page is invalid input, got %v", err)
	}
	if err := module.DeleteGuardian(ctx, peopledirectory.GuardianDelete{GuardianID: 3}); !errors.Is(err, peopledirectory.ErrInvalidGuardian) {
		t.Fatalf("a delete without an actor is invalid input, got %v", err)
	}
	if err := module.SetStudentPayer(ctx, peopledirectory.StudentPayer{ActorAccountID: 1}); !errors.Is(err, peopledirectory.ErrGuardianStudentRequired) {
		t.Fatalf("a payer without a student is refused, got %v", err)
	}
	if len(provider.calls) != 0 {
		t.Fatalf("invalid input must not reach the provider, got %v", provider.calls)
	}
	if len(engine.observed) != 0 {
		t.Fatalf("rejected input is not an observed operation, got %v", engine.observed)
	}

	matches, err := module.SearchGuardians(ctx, "   ", 10)
	if err != nil || len(matches) != 0 || len(provider.calls) != 0 {
		t.Fatalf("a blank search answers empty without asking the provider: %v %v %v", matches, err, provider.calls)
	}
	if _, err := module.SearchGuardians(ctx, "  Meier ", 5); err != nil {
		t.Fatal(err)
	}
	if provider.lastText != "Meier" {
		t.Fatalf("the search text is trimmed before delegation, got %q", provider.lastText)
	}
}

func TestGuardianCallsAreObservedAndErrorsPassThrough(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	provider := &stubProvider{found: peopledirectory.Guardian{ID: 9, FirstName: "Mia"}}
	module.BindGuardianProvider(provider)
	ctx := context.Background()

	guardian, err := module.FindGuardian(ctx, 9)
	if err != nil || guardian.FirstName != "Mia" {
		t.Fatalf("expected the provider's guardian, got %+v %v", guardian, err)
	}
	provider.err = &peopledirectory.GuardianStillLinkedError{StudentNames: []string{"Ben"}}
	err = module.DeleteGuardian(ctx, peopledirectory.GuardianDelete{GuardianID: 9, ActorAccountID: 1})
	var stillLinked *peopledirectory.GuardianStillLinkedError
	if !errors.As(err, &stillLinked) || !errors.Is(err, peopledirectory.ErrGuardianStillLinked) {
		t.Fatalf("provider errors pass through unchanged, got %v", err)
	}
	want := []string{"find_guardian:none", "delete_guardian:guardian_linked"}
	if len(engine.observed) != len(want) || engine.observed[0] != want[0] || engine.observed[1] != want[1] {
		t.Fatalf("expected observations %v, got %v", want, engine.observed)
	}
}

func TestGuardianRowReadsUseTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	ctx := context.Background()

	if _, err := module.ListGuardianLinksByAccount(ctx, 0); !errors.Is(err, peopledirectory.ErrInvalidGuardian) {
		t.Fatalf("an account id is required, got %v", err)
	}
	if _, err := module.ListGuardianLinksByAccount(ctx, 42); err != nil || engine.guardian.accountID != 42 {
		t.Fatalf("the account read reaches the engine: %v %+v", err, engine.guardian)
	}
	guardians, err := module.ListGuardiansByAccount(ctx, []int64{0, -1})
	if err != nil || len(guardians) != 0 || engine.calls != 1 {
		t.Fatalf("no positive ids means no engine call: %v %v %d", guardians, err, engine.calls)
	}
	if _, err := module.ListGuardiansByID(ctx, []int64{3, 3, 4}); err != nil || len(engine.guardian.ids) != 2 {
		t.Fatalf("ids are deduplicated before the engine: %v %+v", err, engine.guardian)
	}
	counts, err := module.CountGuardianLinks(ctx, nil)
	if err != nil || len(counts) != 0 || engine.calls != 2 {
		t.Fatalf("an empty count answers without the engine: %v %v %d", counts, err, engine.calls)
	}
}

func TestGuardianLinkHasPermission(t *testing.T) {
	t.Parallel()
	link := peopledirectory.GuardianLink{Permissions: []string{peopledirectory.GuardianPermissionPortalAccess}}
	if !link.HasPermission(peopledirectory.GuardianPermissionPortalAccess) || link.HasPermission("parent_portal.notes.write") {
		t.Fatal("HasPermission must answer from the granted names only")
	}
	if got := (peopledirectory.Guardian{FirstName: " Mia ", LastName: ""}).FullName(); got != "Mia" {
		t.Fatalf("FullName trims and skips the empty part, got %q", got)
	}
	if !(peopledirectory.NewStudentGuardian{}).CreatesProfile() {
		t.Fatal("an entry without a profile id creates a profile")
	}
}
