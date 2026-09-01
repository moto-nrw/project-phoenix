package models

// Compile-time interface compliance checks for models with the conventional
// ID/created_at/updated_at shape.
//
// If a conventional model fails to implement base.Entity
// (GetID, GetCreatedAt, GetUpdatedAt), this file will fail to compile.

import (
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Compile-time assertions for base.Entity interface compliance
var (
	// active package
	_ base.Entity = (*active.Attendance)(nil)
	_ base.Entity = (*active.CombinedGroup)(nil)
	_ base.Entity = (*active.Group)(nil)
	_ base.Entity = (*active.GroupMapping)(nil)
	_ base.Entity = (*active.GroupSupervisor)(nil)
	_ base.Entity = (*active.Visit)(nil)

	// activities package
	_ base.Entity = (*activities.Category)(nil)
	_ base.Entity = (*activities.Group)(nil)
	_ base.Entity = (*activities.Schedule)(nil)
	_ base.Entity = (*activities.StudentEnrollment)(nil)
	_ base.Entity = (*activities.SupervisorPlanned)(nil)

	// auth package
	_ base.Entity = (*auth.Account)(nil)
	_ base.Entity = (*auth.AccountParent)(nil)
	_ base.Entity = (*auth.AccountPermission)(nil)
	_ base.Entity = (*auth.AccountRole)(nil)
	_ base.Entity = (*auth.GuardianInvitation)(nil)
	_ base.Entity = (*auth.InvitationToken)(nil)
	_ base.Entity = (*auth.PasswordResetToken)(nil)
	_ base.Entity = (*auth.Permission)(nil)
	_ base.Entity = (*auth.Role)(nil)
	_ base.Entity = (*auth.RolePermission)(nil)
	_ base.Entity = (*auth.Token)(nil)

	// education package
	_ base.Entity = (*education.Group)(nil)
	_ base.Entity = (*education.GroupSubstitution)(nil)
	_ base.Entity = (*education.GroupTeacher)(nil)

	// facilities package
	_ base.Entity = (*facilities.Room)(nil)

	// iot package
	_ base.Entity = (*iot.Device)(nil)

	// platform package
	_ base.Entity = (*platform.OperatorRefreshToken)(nil)

	// schedule package
	_ base.Entity = (*schedule.Dateframe)(nil)
	_ base.Entity = (*schedule.RecurrenceRule)(nil)
	_ base.Entity = (*schedule.Timeframe)(nil)

	// users package
	_ base.Entity = (*users.Guest)(nil)
	_ base.Entity = (*users.GuardianProfile)(nil)
	_ base.Entity = (*users.Person)(nil)
	_ base.Entity = (*users.PrivacyConsent)(nil)
	_ base.Entity = (*users.Profile)(nil)
	_ base.Entity = (*users.RFIDCard)(nil)
	_ base.Entity = (*users.Staff)(nil)
	_ base.Entity = (*users.Student)(nil)
	_ base.Entity = (*users.StudentGuardian)(nil)
	_ base.Entity = (*users.Teacher)(nil)
)

type formerGetterShape struct {
	name                                string
	model                               any
	timestampField                      string
	timestampBun                        string
	expectsStringIDModelWithoutNullZero bool
}

func TestFormerModelGetterExceptionsHaveHonestShapes(t *testing.T) {
	t.Parallel()

	tests := []formerGetterShape{
		{name: "auth event", model: (*audit.AuthEvent)(nil), timestampField: "CreatedAt", timestampBun: "created_at,notnull,default:now()"},
		{name: "data access log", model: (*audit.DataAccessLog)(nil), timestampField: "AccessedAt", timestampBun: "accessed_at,notnull,default:now()"},
		{name: "data deletion", model: (*audit.DataDeletion)(nil), timestampField: "DeletedAt", timestampBun: "deleted_at,notnull,default:now()"},
		{name: "deviation event", model: (*audit.DeviationEvent)(nil), timestampField: "OccurredAt", timestampBun: "occurred_at,notnull,default:now()"},
		{name: "enrollment offering adjustment", model: (*audit.EnrollmentOfferingAdjustment)(nil), timestampField: "ChangedAt", timestampBun: "changed_at,notnull,default:now()"},
		{name: "guardian change", model: (*audit.GuardianChange)(nil), timestampField: "ChangedAt", timestampBun: "changed_at,notnull,default:now()"},
		{name: "passkey session", model: (*auth.PasskeySession)(nil), expectsStringIDModelWithoutNullZero: true},
		{name: "operator passkey session", model: (*platform.OperatorPasskeySession)(nil), expectsStringIDModelWithoutNullZero: true},
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
	_, isGenericEntity := reflect.New(modelType).Interface().(base.Entity)
	if shape.expectsStringIDModelWithoutNullZero {
		assertPasskeySessionShape(t, modelType, isGenericEntity)
		return
	}
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

func assertPasskeySessionShape(t *testing.T, modelType reflect.Type, isGenericEntity bool) {
	t.Helper()

	field, ok := modelType.FieldByName("StringIDModelWithoutNullZero")
	if !ok || !field.Anonymous || field.Type != reflect.TypeFor[base.StringIDModelWithoutNullZero]() {
		t.Fatalf("%s must embed base.StringIDModelWithoutNullZero", modelType)
	}
	expectedTags := map[string]string{
		"ID":        "id,pk",
		"CreatedAt": "created_at,notnull,default:current_timestamp",
		"UpdatedAt": "updated_at,notnull,default:current_timestamp",
	}
	for fieldName, expectedTag := range expectedTags {
		baseField, found := field.Type.FieldByName(fieldName)
		if !found || baseField.Tag.Get("bun") != expectedTag {
			t.Fatalf("%s.%s must preserve bun tag %q", modelType, fieldName, expectedTag)
		}
	}
	if !isGenericEntity {
		t.Fatalf("%s must satisfy base.Entity through base.StringIDModelWithoutNullZero", modelType)
	}
}
