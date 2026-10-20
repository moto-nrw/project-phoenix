package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func TestCreatePersonNormalizesNamesAndTag(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)
	tag := " aa:bb-cc "

	_, err := module.CreatePerson(context.Background(), peopledirectory.CreatePerson{FirstName: "  Mia ", LastName: " Muster ", TagID: &tag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.created.FirstName != "Mia" || engine.created.LastName != "Muster" {
		t.Fatalf("names were not trimmed: %+v", engine.created)
	}
	if engine.created.TagID == nil || *engine.created.TagID != "AABBCC" {
		t.Fatalf("tag was not normalized: %v", engine.created.TagID)
	}
}

func TestCreatePersonRejectsMissingNamesWithoutTouchingTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)

	_, err := module.CreatePerson(context.Background(), peopledirectory.CreatePerson{FirstName: " ", LastName: "Muster"})
	if !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("expected ErrInvalidPerson, got %v", err)
	}
	var invalid *peopledirectory.InvalidPersonError
	if !errors.As(err, &invalid) || invalid.Reason != "first name is required" {
		t.Fatalf("expected typed reason, got %v", err)
	}
	if engine.calls != 0 {
		t.Fatalf("engine must not be called for invalid input")
	}
}

func TestUpdatePersonRequiresID(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)

	_, err := module.UpdatePerson(context.Background(), peopledirectory.UpdatePerson{FirstName: "A", LastName: "B"})
	if !errors.Is(err, peopledirectory.ErrInvalidPerson) || engine.calls != 0 {
		t.Fatalf("expected invalid person without engine call, got %v (%d calls)", err, engine.calls)
	}
}

func TestListPersonsByIDDeduplicatesAndSkipsEmptyInput(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)

	result, err := module.ListPersonsByID(context.Background(), []int64{0, -1})
	if err != nil || len(result) != 0 || engine.calls != 0 {
		t.Fatalf("empty input must short-circuit: %v %v %d", result, err, engine.calls)
	}
	if _, err := module.ListPersonsByID(context.Background(), []int64{3, 3, 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(engine.listed) != 2 || engine.listed[0] != 3 || engine.listed[1] != 1 || engine.across {
		t.Fatalf("expected deduplicated tenant-scoped list, got %v across=%v", engine.listed, engine.across)
	}
	if _, err := module.ListPersonsAcrossTenantsByID(context.Background(), []int64{5, 5}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(engine.listed) != 1 || !engine.across {
		t.Fatalf("expected the cross-tenant engine call, got %v across=%v", engine.listed, engine.across)
	}
	if _, err := module.ListPersonsByTenantIDs(context.Background(), []int64{7, 7}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(engine.listed) != 1 || engine.listed[0] != 7 {
		t.Fatalf("expected the deduplicated tenant list, got %v", engine.listed)
	}
}

func TestSearchPersonsCapsPageSizeAndTrimsFilters(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)

	_, err := module.SearchPersons(context.Background(), peopledirectory.PersonFilter{FirstNamePrefix: " Mi ", TagID: " aa:bb-cc ", PageSize: 5000, Page: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.searched.FirstNamePrefix != "Mi" || engine.searched.TagID != "AABBCC" || engine.searched.PageSize != peopledirectory.MaxPageSize || engine.searched.Page != 2 {
		t.Fatalf("filter was not normalized: %+v", engine.searched)
	}
	if _, err := module.SearchPersons(context.Background(), peopledirectory.PersonFilter{Page: -1}); !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("negative page must be rejected, got %v", err)
	}
}

func TestTagCommandsNormalizeAndValidate(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)

	if err := module.LinkTag(context.Background(), 7, "ab:cd"); err != nil || engine.tag != "ABCD" {
		t.Fatalf("expected normalized tag, got %q err=%v", engine.tag, err)
	}
	if err := module.LinkTag(context.Background(), 7, "  "); !errors.Is(err, peopledirectory.ErrInvalidPerson) {
		t.Fatalf("empty tag must be rejected, got %v", err)
	}
	restored, err := module.RestoreTag(context.Background(), 7, "")
	if err != nil || restored {
		t.Fatalf("an empty tag restores nothing: %v %v", restored, err)
	}
	if _, err := module.FindPersonByTag(context.Background(), " x-y "); err != nil || engine.tag != "XY" {
		t.Fatalf("lookup must normalize the tag, got %q err=%v", engine.tag, err)
	}
}
