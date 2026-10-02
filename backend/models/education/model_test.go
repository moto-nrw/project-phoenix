package education

import (
	"reflect"
	"testing"
)

// The School Structure rows carry the shared identity, timestamp and tenant
// columns themselves (#2742). Their mapping must stay the shared base shape
// column for column, or the rows stop reading and writing as before.
func TestModelMapsTheSharedRowColumns(t *testing.T) {
	t.Parallel()

	want := map[string][2]string{
		"ID":        {"id,pk,autoincrement", "id"},
		"CreatedAt": {"created_at,nullzero,notnull,default:current_timestamp", "created_at"},
		"UpdatedAt": {"updated_at,nullzero,notnull,default:current_timestamp", "updated_at"},
		"TenantID":  {"tenant_id,notnull", "tenant_id"},
	}
	for _, row := range []any{Group{}, GroupTeacher{}, ClassTeacher{}, GroupSubstitution{}, GradeTransition{}, GradeTransitionMapping{}, GradeTransitionHistory{}} {
		rowType := reflect.TypeOf(row)
		for name, tags := range want {
			field, found := rowType.FieldByName(name)
			if !found {
				t.Errorf("%s has no %s column", rowType.Name(), name)
				continue
			}
			if got := field.Tag.Get("bun"); got != tags[0] {
				t.Errorf("%s.%s bun tag = %q, want %q", rowType.Name(), name, got, tags[0])
			}
			if got := field.Tag.Get("json"); got != tags[1] {
				t.Errorf("%s.%s json tag = %q, want %q", rowType.Name(), name, got, tags[1])
			}
		}
	}
}

func TestTenantModelAccessors(t *testing.T) {
	t.Parallel()

	group := &Group{}
	group.SetTenantID(7)
	if group.GetTenantID() != 7 || group.TenantID != 7 {
		t.Fatalf("tenant = %d, want 7", group.GetTenantID())
	}
}

func TestIsNotFoundRecognisesTheMarker(t *testing.T) {
	t.Parallel()

	if !IsNotFound(&DatabaseError{Op: "find", Err: ErrNotFound}) {
		t.Fatal("a wrapped ErrNotFound must classify as not found")
	}
	if IsNotFound(&DatabaseError{Op: "find"}) {
		t.Fatal("a database error without a missing row is not a not-found")
	}
}
