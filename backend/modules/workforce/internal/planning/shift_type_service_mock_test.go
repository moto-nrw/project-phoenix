package planning

// Mock-based unit tests for the ShiftTypeService (Schichtarten, #1836). They
// drive the validation, not-found, name-conflict and propagation branches that
// the DB-backed hermetic tests don't reach. int64 literals are fake in-memory
// IDs, not DB rows.

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"
)

// ----------------------------------------------------------------------------
// Mock repository
// ----------------------------------------------------------------------------

// errRowNotFound is the missing-row answer of the row adapters as the
// services see it: an error carrying the repository not-found marker.
var errRowNotFound error = rowNotFoundError{}

type rowNotFoundError struct{}

func (rowNotFoundError) Error() string       { return "repository: not found" }
func (rowNotFoundError) RepositoryNotFound() {}

type shiftTypeMockRepo struct {
	createFunc        func(ctx context.Context, st *workforce.ShiftTypeRecord) error
	findByIDFunc      func(ctx context.Context, id any) (*workforce.ShiftTypeRecord, error)
	updateFunc        func(ctx context.Context, st *workforce.ShiftTypeRecord) error
	deleteFunc        func(ctx context.Context, id any) error
	listAllFunc       func(ctx context.Context) ([]*workforce.ShiftTypeRecord, error)
	createIfAbsentFn  func(ctx context.Context, st *workforce.ShiftTypeRecord) (bool, error)
	createIfAbsentHit int
}

func (m *shiftTypeMockRepo) Create(ctx context.Context, st *workforce.ShiftTypeRecord) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, st)
	}
	return nil
}

func (m *shiftTypeMockRepo) FindByID(ctx context.Context, id any) (*workforce.ShiftTypeRecord, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errRowNotFound
}

func (m *shiftTypeMockRepo) Update(ctx context.Context, st *workforce.ShiftTypeRecord) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, st)
	}
	return nil
}

func (m *shiftTypeMockRepo) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *shiftTypeMockRepo) ListAll(ctx context.Context) ([]*workforce.ShiftTypeRecord, error) {
	if m.listAllFunc != nil {
		return m.listAllFunc(ctx)
	}
	return nil, nil
}

func (m *shiftTypeMockRepo) CreateIfAbsent(ctx context.Context, st *workforce.ShiftTypeRecord) (bool, error) {
	m.createIfAbsentHit++
	if m.createIfAbsentFn != nil {
		return m.createIfAbsentFn(ctx, st)
	}
	return true, nil
}

func validShiftType() *workforce.ShiftTypeRecord {
	return &workforce.ShiftTypeRecord{Name: "Betreuung", Color: "#83CD2D", IsActive: true}
}

// ----------------------------------------------------------------------------
// GetShiftType
// ----------------------------------------------------------------------------

func TestShiftTypeService_GetShiftType_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("non-positive id is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{}, nil)
		_, err := svc.GetShiftType(ctx, 0)
		require.ErrorIs(t, err, ErrShiftTypeNotFound)
	})

	t.Run("a missing row maps to not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, errRowNotFound
			},
		}, nil)
		_, err := svc.GetShiftType(ctx, 5)
		require.ErrorIs(t, err, ErrShiftTypeNotFound)
	})

	t.Run("generic repo error is wrapped", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, errors.New("db down")
			},
		}, nil)
		_, err := svc.GetShiftType(ctx, 5)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrShiftTypeNotFound)
		assert.Contains(t, err.Error(), "find shift type")
	})

	t.Run("nil row is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, nil
			},
		}, nil)
		_, err := svc.GetShiftType(ctx, 5)
		require.ErrorIs(t, err, ErrShiftTypeNotFound)
	})

	t.Run("success returns the row", func(t *testing.T) {
		want := validShiftType()
		want.ID = 5
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return want, nil
			},
		}, nil)
		got, err := svc.GetShiftType(ctx, 5)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

func TestShiftTypeService_ListShiftTypes_Delegates(t *testing.T) {
	t.Parallel()

	rows := []*workforce.ShiftTypeRecord{validShiftType()}
	svc := NewShiftTypeService(&shiftTypeMockRepo{
		listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) { return rows, nil },
	}, nil)
	got, err := svc.ListShiftTypes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, rows, got)
}

// ----------------------------------------------------------------------------
// CreateShiftType
// ----------------------------------------------------------------------------

func TestShiftTypeService_CreateShiftType_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("invalid input is rejected before hitting the repo", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{}, nil)
		_, err := svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: "   "})
		require.ErrorIs(t, err, ErrShiftTypeInvalid)
	})

	t.Run("name-check repo error propagates", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) {
				return nil, errors.New("list failed")
			},
		}, nil)
		_, err := svc.CreateShiftType(ctx, validShiftType())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("duplicate name is rejected", func(t *testing.T) {
		existing := validShiftType()
		existing.ID = 9
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) {
				return []*workforce.ShiftTypeRecord{existing}, nil
			},
		}, nil)
		_, err := svc.CreateShiftType(ctx, validShiftType())
		require.ErrorIs(t, err, ErrShiftTypeNameTaken)
	})

	t.Run("non-conflict repo error propagates verbatim", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			createFunc: func(context.Context, *workforce.ShiftTypeRecord) error {
				return errors.New("insert boom")
			},
		}, nil)
		_, err := svc.CreateShiftType(ctx, validShiftType())
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrShiftTypeNameTaken)
		assert.Contains(t, err.Error(), "insert boom")
	})

	t.Run("success returns the created row", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			createFunc: func(_ context.Context, st *workforce.ShiftTypeRecord) error {
				st.ID = 11
				return nil
			},
		}, nil)
		got, err := svc.CreateShiftType(ctx, validShiftType())
		require.NoError(t, err)
		assert.Equal(t, int64(11), got.ID)
	})
}

// ----------------------------------------------------------------------------
// UpdateShiftType
// ----------------------------------------------------------------------------

func TestShiftTypeService_UpdateShiftType_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	withID := func(id int64) *workforce.ShiftTypeRecord {
		st := validShiftType()
		st.ID = id
		return st
	}

	t.Run("non-positive id is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(0))
		require.ErrorIs(t, err, ErrShiftTypeNotFound)
	})

	t.Run("missing row is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, errRowNotFound
			},
		}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(5))
		require.ErrorIs(t, err, ErrShiftTypeNotFound)
	})

	t.Run("lookup error is wrapped", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, errors.New("db down")
			},
		}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(5))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "find shift type")
	})

	t.Run("nil existing row is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, nil
			},
		}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(5))
		require.ErrorIs(t, err, ErrShiftTypeNotFound)
	})

	t.Run("invalid input is rejected", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return withID(5), nil
			},
		}, nil)
		bad := withID(5)
		bad.Name = "   "
		_, err := svc.UpdateShiftType(ctx, bad)
		require.ErrorIs(t, err, ErrShiftTypeInvalid)
	})

	t.Run("name-check error propagates", func(t *testing.T) {
		calls := 0
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return withID(5), nil
			},
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) {
				calls++
				return nil, errors.New("list failed")
			},
		}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(5))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
		assert.Equal(t, 1, calls)
	})

	t.Run("duplicate name on another row is rejected", func(t *testing.T) {
		other := validShiftType()
		other.ID = 99
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return withID(5), nil
			},
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) {
				return []*workforce.ShiftTypeRecord{other}, nil
			},
		}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(5))
		require.ErrorIs(t, err, ErrShiftTypeNameTaken)
	})

	t.Run("non-conflict update error propagates", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return withID(5), nil
			},
			updateFunc: func(context.Context, *workforce.ShiftTypeRecord) error {
				return errors.New("update boom")
			},
		}, nil)
		_, err := svc.UpdateShiftType(ctx, withID(5))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrShiftTypeNameTaken)
		assert.Contains(t, err.Error(), "update boom")
	})

	t.Run("success preserves the existing tenant id", func(t *testing.T) {
		existing := withID(5)
		existing.TenantID = 77
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return existing, nil
			},
		}, nil)
		in := withID(5)
		in.TenantID = 0
		got, err := svc.UpdateShiftType(ctx, in)
		require.NoError(t, err)
		assert.Equal(t, int64(77), got.TenantID)
	})
}

// ----------------------------------------------------------------------------
// DeleteShiftType
// ----------------------------------------------------------------------------

func TestShiftTypeService_DeleteShiftType_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	existing := validShiftType()
	existing.ID = 5

	t.Run("non-positive id is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{}, nil)
		require.ErrorIs(t, svc.DeleteShiftType(ctx, 0), ErrShiftTypeNotFound)
	})

	t.Run("missing row is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, errRowNotFound
			},
		}, nil)
		require.ErrorIs(t, svc.DeleteShiftType(ctx, 5), ErrShiftTypeNotFound)
	})

	t.Run("lookup error is wrapped", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, errors.New("db down")
			},
		}, nil)
		err := svc.DeleteShiftType(ctx, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "find shift type")
	})

	t.Run("nil existing row is not found", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return nil, nil
			},
		}, nil)
		require.ErrorIs(t, svc.DeleteShiftType(ctx, 5), ErrShiftTypeNotFound)
	})

	t.Run("delete error propagates", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return existing, nil
			},
			deleteFunc: func(context.Context, any) error { return errors.New("delete boom") },
		}, nil)
		err := svc.DeleteShiftType(ctx, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete boom")
	})

	t.Run("success", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			findByIDFunc: func(context.Context, any) (*workforce.ShiftTypeRecord, error) {
				return existing, nil
			},
		}, nil)
		require.NoError(t, svc.DeleteShiftType(ctx, 5))
	})
}

// ----------------------------------------------------------------------------
// CreateDefaultShiftTypes
// ----------------------------------------------------------------------------

func TestShiftTypeService_CreateDefaults_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("initial list error propagates", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) {
				return nil, errors.New("list failed")
			},
		}, nil)
		_, err := svc.CreateDefaultShiftTypes(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("create-if-absent error propagates", func(t *testing.T) {
		svc := NewShiftTypeService(&shiftTypeMockRepo{
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) { return nil, nil },
			createIfAbsentFn: func(context.Context, *workforce.ShiftTypeRecord) (bool, error) {
				return false, errors.New("seed boom")
			},
		}, nil)
		_, err := svc.CreateDefaultShiftTypes(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "seed boom")
	})

	t.Run("skips defaults whose name already exists", func(t *testing.T) {
		present := &workforce.ShiftTypeRecord{Name: "Pause", Color: "#6B7280", IsActive: true}
		repo := &shiftTypeMockRepo{
			listAllFunc: func(context.Context) ([]*workforce.ShiftTypeRecord, error) {
				return []*workforce.ShiftTypeRecord{present}, nil
			},
		}
		svc := NewShiftTypeService(repo, nil)
		_, err := svc.CreateDefaultShiftTypes(ctx)
		require.NoError(t, err)
		// Five defaults, "Pause" already present → only four inserts attempted.
		assert.Equal(t, 4, repo.createIfAbsentHit)
	})
}

// ----------------------------------------------------------------------------
// isShiftTypeNameConflict
// ----------------------------------------------------------------------------

func TestIsShiftTypeNameConflict(t *testing.T) {
	t.Parallel()

	assert.False(t, isShiftTypeNameConflict(nil), "nil is never a conflict")
	assert.False(t, isShiftTypeNameConflict(errors.New("plain")), "a plain error is not a pg conflict")
	// A bare driver error is not the Workforce store's name conflict.
	assert.False(t, isShiftTypeNameConflict(pgdriver.Error{}), "a pg error without 23505 is not a conflict")
}

func (m *shiftTypeMockRepo) CreateShiftType(ctx context.Context, v workforce.ShiftType) (workforce.ShiftType, error) {
	row := shiftTypeToRow(v)
	err := m.Create(ctx, row)
	return shiftTypeRowToCapability(row), err
}
func (m *shiftTypeMockRepo) FindShiftType(ctx context.Context, id int64) (workforce.ShiftType, error) {
	row, err := m.FindByID(ctx, id)
	if row == nil {
		if err == nil {
			err = workforce.ErrShiftTypeNotFound
		}
		return workforce.ShiftType{}, err
	}
	return shiftTypeRowToCapability(row), err
}
func (m *shiftTypeMockRepo) UpdateShiftType(ctx context.Context, v workforce.ShiftType) (workforce.ShiftType, error) {
	row := shiftTypeToRow(v)
	err := m.Update(ctx, row)
	return shiftTypeRowToCapability(row), err
}
func (m *shiftTypeMockRepo) DeleteShiftType(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *shiftTypeMockRepo) ListShiftTypes(ctx context.Context) ([]workforce.ShiftType, error) {
	rows, err := m.ListAll(ctx)
	result := make([]workforce.ShiftType, 0, len(rows))
	for _, row := range rows {
		result = append(result, shiftTypeRowToCapability(row))
	}
	return result, err
}
func (m *shiftTypeMockRepo) CreateShiftTypeIfAbsent(ctx context.Context, v workforce.ShiftType) (workforce.ShiftType, bool, error) {
	row := shiftTypeToRow(v)
	inserted, err := m.CreateIfAbsent(ctx, row)
	return shiftTypeRowToCapability(row), inserted, err
}
