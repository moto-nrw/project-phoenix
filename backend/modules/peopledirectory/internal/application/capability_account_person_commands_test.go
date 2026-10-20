package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The profile flow's person keeps its names as typed apart from the
// surrounding spaces: no length cap, both required.
func TestCreateAccountPersonTrimsWithoutCapping(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := newValidationService(engine)
	longName := strings.Repeat("a", peopledirectory.MaxNameLength+1)
	if err := module.CreateAccountPerson(context.Background(), 41, "  "+longName+" ", " Beispiel "); err != nil {
		t.Fatalf("create account person: %v", err)
	}
	if engine.created.FirstName != longName || engine.created.LastName != "Beispiel" {
		t.Errorf("names = %q / %q, want trimmed and uncapped", engine.created.FirstName, engine.created.LastName)
	}
	if engine.created.AccountID == nil || *engine.created.AccountID != 41 {
		t.Errorf("account = %v, want 41", engine.created.AccountID)
	}

	err := module.CreateAccountPerson(context.Background(), 41, "", "Beispiel")
	if !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("empty first name: %v, want ErrInvalidPerson", err)
	}
}

// A rename never blanks a name; a nil name is left to the stored one.
func TestRenamePersonRefusesAnEmptyName(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := newValidationService(engine)
	empty, name := "", "Kim"
	if err := module.RenamePerson(context.Background(), 5, &empty, nil); !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("empty first name: %v, want ErrInvalidPerson", err)
	}
	if err := module.RenamePerson(context.Background(), 5, nil, &empty); !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("empty last name: %v, want ErrInvalidPerson", err)
	}
	if engine.calls != 0 {
		t.Fatalf("a refused rename reached the engine %d times", engine.calls)
	}
	if err := module.RenamePerson(context.Background(), 5, &name, nil); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if engine.calls != 1 {
		t.Fatalf("rename reached the engine %d times, want 1", engine.calls)
	}
}

// A staff record or Stammdaten write keeps both names, trimmed and uncapped.
func TestSetPersonDetailsRequiresBothNames(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := newValidationService(engine)
	if err := module.SetPersonDetails(context.Background(), 5, peopledirectory.PersonDetails{FirstName: "", LastName: "Kraft"}); !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("empty first name: %v, want ErrInvalidPerson", err)
	}
	if engine.calls != 0 {
		t.Fatalf("a refused write reached the engine")
	}
	longName := strings.Repeat("b", peopledirectory.MaxNameLength+1)
	details := peopledirectory.PersonDetails{FirstName: " " + longName, LastName: " Kraft ", Birthday: "2015-04-01"}
	if err := module.SetPersonDetails(context.Background(), 5, details); err != nil {
		t.Fatalf("set details: %v", err)
	}
	want := peopledirectory.PersonDetails{FirstName: longName, LastName: "Kraft", Birthday: "2015-04-01"}
	if engine.details != want {
		t.Errorf("details = %+v, want %+v", engine.details, want)
	}
}
