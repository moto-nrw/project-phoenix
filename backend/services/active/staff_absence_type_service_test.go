package active

import (
	"context"
	"errors"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/require"
)

// absTypeRepoMock is an in-memory StaffAbsenceTypeRepository. Only the three
// methods the service uses are behavioural; the rest satisfy the interface.
type absTypeRepoMock struct {
	rows   []*activeModels.StaffAbsenceType
	inUse  map[int64]bool
	nextID int64
}

func (m *absTypeRepoMock) ListAll(_ context.Context) ([]*activeModels.StaffAbsenceType, error) {
	out := make([]*activeModels.StaffAbsenceType, len(m.rows))
	copy(out, m.rows)
	return out, nil
}

func (m *absTypeRepoMock) FindByID(_ context.Context, id any) (*activeModels.StaffAbsenceType, error) {
	wanted, ok := id.(int64)
	if !ok {
		return nil, nil
	}
	for _, r := range m.rows {
		if r.ID == wanted {
			row := *r
			return &row, nil
		}
	}
	return nil, nil
}

func (m *absTypeRepoMock) LockByID(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error) {
	return m.FindByID(ctx, id)
}

func (m *absTypeRepoMock) Create(_ context.Context, at *activeModels.StaffAbsenceType) error {
	if err := at.Validate(); err != nil {
		return err
	}
	m.nextID++
	at.ID = m.nextID
	row := *at
	m.rows = append(m.rows, &row)
	return nil
}

func (m *absTypeRepoMock) Update(_ context.Context, at *activeModels.StaffAbsenceType) error {
	for i, r := range m.rows {
		if r.ID == at.ID {
			row := *at
			m.rows[i] = &row
			return nil
		}
	}
	return errors.New("not found")
}

func (m *absTypeRepoMock) Delete(_ context.Context, _ any) error { return nil }
func (m *absTypeRepoMock) List(_ context.Context, _ *base.QueryOptions) ([]*activeModels.StaffAbsenceType, error) {
	return m.ListAll(context.Background())
}
func (m *absTypeRepoMock) IsInUse(_ context.Context, id int64) (bool, error) {
	return m.inUse[id], nil
}
func (m *absTypeRepoMock) Count(_ context.Context, _ map[string]any) (int, error) {
	return len(m.rows), nil
}

func newAbsenceTypeService() (StaffAbsenceTypeService, *absTypeRepoMock) {
	repo := &absTypeRepoMock{}
	return NewStaffAbsenceTypeService(repo, nil), repo
}

func TestCreateAbsenceTypeTrimsAndPinsBaseType(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()

	created, err := svc.CreateAbsenceType(context.Background(), "  Regenerationstag ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Name != "Regenerationstag" {
		t.Errorf("expected trimmed name, got %q", created.Name)
	}
	if created.BaseType != activeModels.AbsenceTypeOther {
		t.Errorf("expected base type %q, got %q", activeModels.AbsenceTypeOther, created.BaseType)
	}
	if !created.IsActive {
		t.Error("a newly added art must be usable right away")
	}
}

func TestCreateAbsenceTypeRejectsDuplicateIgnoringCaseAndSpace(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	if _, err := svc.CreateAbsenceType(ctx, "Ferienzeit"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.CreateAbsenceType(ctx, "  ferienZEIT "); !errors.Is(err, ErrAbsenceTypeNameTaken) {
		t.Errorf("expected ErrAbsenceTypeNameTaken, got %v", err)
	}
}

func TestCreateAbsenceTypeRejectsStandardTypeNames(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	// A second "Urlaub" that does not touch the Urlaubskontingent would be
	// indistinguishable in the dropdown from the real one.
	for _, name := range []string{"Urlaub", "krank", "Fortbildung", "SONSTIGE", "Freizeitausgleich"} {
		if _, err := svc.CreateAbsenceType(ctx, name); !errors.Is(err, ErrAbsenceTypeNameReserved) {
			t.Errorf("expected ErrAbsenceTypeNameReserved for %q, got %v", name, err)
		}
	}
}

func TestCreateAbsenceTypeRejectsEmptyName(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	if _, err := svc.CreateAbsenceType(context.Background(), "   "); !errors.Is(err, ErrAbsenceTypeInvalid) {
		t.Errorf("expected ErrAbsenceTypeInvalid, got %v", err)
	}
}

func TestUpdateAbsenceTypeRenamesWithoutTouchingActiveFlag(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	created, err := svc.CreateAbsenceType(ctx, "Ferienzeit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inactive := false
	if _, err := svc.UpdateAbsenceType(ctx, created.ID, nil, &inactive); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	renamed := "Ferienbetreuung"
	updated, err := svc.UpdateAbsenceType(ctx, created.ID, &renamed, nil)
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

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	if _, err := svc.CreateAbsenceType(ctx, "Ferienzeit"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.CreateAbsenceType(ctx, "Regenerationstag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clash := "ferienzeit"
	if _, err := svc.UpdateAbsenceType(ctx, second.ID, &clash, nil); !errors.Is(err, ErrAbsenceTypeNameTaken) {
		t.Errorf("expected ErrAbsenceTypeNameTaken, got %v", err)
	}
}

func TestUpdateAbsenceTypeAllowsRenamingToItsOwnName(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	created, err := svc.CreateAbsenceType(ctx, "Ferienzeit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	same := "Ferienzeit"
	if _, err := svc.UpdateAbsenceType(ctx, created.ID, &same, nil); err != nil {
		t.Errorf("renaming an art to its own name must be a no-op, got %v", err)
	}
}

func TestUpdateAbsenceTypeRejectsRenamingUsedType(t *testing.T) {
	t.Parallel()

	svc, repo := newAbsenceTypeService()
	ctx := context.Background()

	created, err := svc.CreateAbsenceType(ctx, "Regenerationstag")
	require.NoError(t, err)
	repo.inUse = map[int64]bool{created.ID: true}

	renamed := "Gesundheitstag"
	_, err = svc.UpdateAbsenceType(ctx, created.ID, &renamed, nil)
	require.ErrorIs(t, err, ErrAbsenceTypeInUse)
}

func TestResolveForAbsenceRejectsDeactivatedArt(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	created, err := svc.CreateAbsenceType(ctx, "Sonderurlaub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inactive := false
	if _, err := svc.UpdateAbsenceType(ctx, created.ID, nil, &inactive); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.ResolveForAbsence(ctx, created.ID); !errors.Is(err, ErrAbsenceTypeInactive) {
		t.Errorf("expected ErrAbsenceTypeInactive, got %v", err)
	}
}

func TestResolveForAbsenceRejectsUnknownID(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	if _, err := svc.ResolveForAbsence(context.Background(), 4711); !errors.Is(err, ErrAbsenceTypeNotFound) {
		t.Errorf("expected ErrAbsenceTypeNotFound, got %v", err)
	}
}

func TestStampAbsenceTypeLabelsFillsOnlyCustomRows(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	ctx := context.Background()

	created, err := svc.CreateAbsenceType(ctx, "Regenerationstag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	custom := &activeModels.StaffAbsence{AbsenceType: activeModels.AbsenceTypeOther, AbsenceTypeID: &created.ID}
	standard := &activeModels.StaffAbsence{AbsenceType: activeModels.AbsenceTypeSick}
	StampAbsenceTypeLabels(ctx, svc, []*activeModels.StaffAbsence{custom, standard, nil})

	if custom.AbsenceTypeLabel != "Regenerationstag" {
		t.Errorf("expected the school's own wording, got %q", custom.AbsenceTypeLabel)
	}
	if standard.AbsenceTypeLabel != "" {
		t.Errorf("a standard type must stay unlabelled so clients use their own label, got %q", standard.AbsenceTypeLabel)
	}
}

func TestStampAbsenceTypeLabelsIsNilSafe(t *testing.T) {
	t.Parallel()

	svc, _ := newAbsenceTypeService()
	created, err := svc.CreateAbsenceType(context.Background(), "Regenerationstag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	absence := &activeModels.StaffAbsence{AbsenceTypeID: &created.ID}
	StampAbsenceTypeLabels(context.Background(), nil, []*activeModels.StaffAbsence{absence})
	if absence.AbsenceTypeLabel != "" {
		t.Error("without a type service nothing may be stamped")
	}
}
