package records

import (
	"reflect"
	"testing"
	"time"
)

// genericEntity is the method set of the legacy models' base.Entity; the
// audit records keep their own identity and timestamp mapping instead.
type genericEntity interface {
	GetID() interface{}
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

type formerGetterShape struct {
	name           string
	model          any
	timestampField string
	timestampBun   string
}

func TestFormerModelGetterExceptionsHaveHonestShapes(t *testing.T) {
	t.Parallel()

	tests := []formerGetterShape{
		{name: "auth event", model: (*AuthEvent)(nil), timestampField: "CreatedAt", timestampBun: "created_at,notnull,default:now()"},
		{name: "data access log", model: (*DataAccessLog)(nil), timestampField: "AccessedAt", timestampBun: "accessed_at,notnull,default:now()"},
		{name: "data deletion", model: (*DataDeletion)(nil), timestampField: "DeletedAt", timestampBun: "deleted_at,notnull,default:now()"},
		{name: "deviation event", model: (*DeviationEvent)(nil), timestampField: "OccurredAt", timestampBun: "occurred_at,notnull,default:now()"},
		{name: "enrollment offering adjustment", model: (*EnrollmentOfferingAdjustment)(nil), timestampField: "ChangedAt", timestampBun: "changed_at,notnull,default:now()"},
		{name: "guardian change", model: (*GuardianChange)(nil), timestampField: "ChangedAt", timestampBun: "changed_at,notnull,default:now()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertFormerGetterShape(t, tt)
		})
	}
}

func assertFormerGetterShape(t *testing.T, shape formerGetterShape) {
	t.Helper()

	modelType := reflect.TypeOf(shape.model).Elem()
	_, isGenericEntity := reflect.New(modelType).Interface().(genericEntity)
	if isGenericEntity {
		t.Fatalf("%s must not map its audit timestamp to the generic Entity contract", modelType)
	}
	idField, ok := modelType.FieldByName("ID")
	if !ok || len(idField.Index) != 1 || idField.Tag.Get("bun") != "id,pk,autoincrement" {
		t.Fatalf("%s must keep its direct audit identity mapping", modelType)
	}
	timestampField, ok := modelType.FieldByName(shape.timestampField)
	if !ok || timestampField.Tag.Get("bun") != shape.timestampBun {
		t.Fatalf("%s must map %s with bun tag %q", modelType, shape.timestampField, shape.timestampBun)
	}
}
