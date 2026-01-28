package models

// Compile-time interface compliance checks
// These assertions verify that all models correctly implement base.Entity
// at compile time, eliminating the need for 44 separate runtime tests.
//
// If any model fails to implement base.Entity (GetID, GetCreatedAt, GetUpdatedAt),
// this file will fail to compile.

import (
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	"github.com/moto-nrw/project-phoenix/models/iot"
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

	// audit package
	_ base.Entity = (*audit.AuthEvent)(nil)
	_ base.Entity = (*audit.DataDeletion)(nil)

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

	// feedback package
	_ base.Entity = (*feedback.Entry)(nil)

	// iot package
	_ base.Entity = (*iot.Device)(nil)

	// schedule package
	_ base.Entity = (*schedule.Dateframe)(nil)
	_ base.Entity = (*schedule.RecurrenceRule)(nil)
	_ base.Entity = (*schedule.Timeframe)(nil)

	// users package
	_ base.Entity = (*users.Guest)(nil)
	_ base.Entity = (*users.GuardianProfile)(nil)
	_ base.Entity = (*users.Person)(nil)
	_ base.Entity = (*users.PersonGuardian)(nil)
	_ base.Entity = (*users.PrivacyConsent)(nil)
	_ base.Entity = (*users.Profile)(nil)
	_ base.Entity = (*users.RFIDCard)(nil)
	_ base.Entity = (*users.Staff)(nil)
	_ base.Entity = (*users.Student)(nil)
	_ base.Entity = (*users.StudentGuardian)(nil)
	_ base.Entity = (*users.Teacher)(nil)
)
