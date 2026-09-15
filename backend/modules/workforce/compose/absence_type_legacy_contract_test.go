package compose

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCreateAbsenceTypeTrimsAndPinsBaseType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)

	created, err := svc.CreateAbsenceType(testpkg.Ctx(t), workforce.CreateAbsenceType{Name: "  Regenerationstag "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Name != "Regenerationstag" {
		t.Errorf("expected trimmed name, got %q", created.Name)
	}
	if created.BaseType != workforce.AbsenceTypeOther {
		t.Errorf("expected base type %q, got %q", workforce.AbsenceTypeOther, created.BaseType)
	}
	if !created.IsActive {
		t.Error("a newly added art must be usable right away")
	}
}

func TestCreateAbsenceTypeRejectsDuplicateIgnoringCaseAndSpace(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	if _, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Ferienzeit"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "  ferienZEIT "}); !errors.Is(err, workforce.ErrAbsenceTypeNameTaken) {
		t.Errorf("expected workforce.ErrAbsenceTypeNameTaken, got %v", err)
	}
}

func TestCreateAbsenceTypeRejectsStandardTypeNames(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	// A second "Urlaub" that does not touch the Urlaubskontingent would be
	// indistinguishable in the dropdown from the real one.
	for _, name := range []string{"Urlaub", "krank", "Fortbildung", "SONSTIGE", "Freizeitausgleich"} {
		if _, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: name}); !errors.Is(err, workforce.ErrAbsenceTypeNameReserved) {
			t.Errorf("expected workforce.ErrAbsenceTypeNameReserved for %q, got %v", name, err)
		}
	}
}

func TestCreateAbsenceTypeRejectsEmptyName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	if _, err := svc.CreateAbsenceType(testpkg.Ctx(t), workforce.CreateAbsenceType{Name: "   "}); !errors.Is(err, workforce.ErrAbsenceTypeInvalid) {
		t.Errorf("expected workforce.ErrAbsenceTypeInvalid, got %v", err)
	}
}

func TestUpdateAbsenceTypeRenamesWithoutTouchingActiveFlag(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Ferienzeit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inactive := false
	if _, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: nil, IsActive: &inactive}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	renamed := "Ferienbetreuung"
	updated, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &renamed, IsActive: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Ferienbetreuung" {
		t.Errorf("expected renamed art, got %q", updated.Name)
	}
	if updated.IsActive {
		t.Error("a rename must not silently reactivate a retired art")
	}
}

func TestUpdateAbsenceTypeRejectsRenameOntoAnotherName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	if _, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Ferienzeit"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Regenerationstag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clash := "ferienzeit"
	if _, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: second.ID, Name: &clash, IsActive: nil}); !errors.Is(err, workforce.ErrAbsenceTypeNameTaken) {
		t.Errorf("expected workforce.ErrAbsenceTypeNameTaken, got %v", err)
	}
}

func TestUpdateAbsenceTypeAllowsRenamingToItsOwnName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Ferienzeit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	same := "Ferienzeit"
	if _, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &same, IsActive: nil}); err != nil {
		t.Errorf("renaming an art to its own name must be a no-op, got %v", err)
	}
}

func TestUpdateAbsenceTypeRejectsRenamingUsedType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Regenerationstag"})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Used", "Type")
	_, err = svc.CreateStaffAbsence(ctx, workforce.StaffAbsence{StaffID: staff.ID, CreatedBy: staff.ID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &created.ID, Status: workforce.AbsenceStatusReported, DateStart: "2026-09-07", DateEnd: "2026-09-07"})
	require.NoError(t, err)

	renamed := "Gesundheitstag"
	_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &renamed, IsActive: nil})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeInUse)
}
