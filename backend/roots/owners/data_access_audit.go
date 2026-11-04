package owners

import (
	"context"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

type dataAccessAudit struct {
	writer DataAccessAppender
}

// NewDataAccessAudit connects attendance access evidence to the audit writer.
func NewDataAccessAudit(writer DataAccessAppender) presenceservice.DataAccessAudit {
	if writer == nil {
		return nil
	}
	return dataAccessAudit{writer: writer}
}

func (a dataAccessAudit) Create(ctx context.Context, event *presenceservice.DataAccessEvent) error {
	if event == nil {
		return a.writer.Create(ctx, nil)
	}
	return a.writer.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: event.ActorAccountID, ActorRole: event.ActorRole,
		ResourceType: event.ResourceType, StudentID: event.StudentID,
		RangeStart: event.RangeStart, RangeEnd: event.RangeEnd,
		AccessedAt: event.AccessedAt, Metadata: event.Metadata,
	})
}

// NewTimeTrackingDataAccessAudit connects the retained time export's access
// evidence to the audit writer.
func NewTimeTrackingDataAccessAudit(writer DataAccessAppender) workforceCompose.DataAccessAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingDataAccessAudit{writer: writer}
}

type timeTrackingDataAccessAudit struct {
	writer DataAccessAppender
}

func (a timeTrackingDataAccessAudit) Create(ctx context.Context, event *workforceCompose.DataAccessEvent) error {
	if event == nil {
		return a.writer.Create(ctx, nil)
	}
	return a.writer.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: event.ActorAccountID, ActorRole: event.ActorRole,
		ResourceType: event.ResourceType, StudentID: event.StudentID,
		RangeStart: event.RangeStart, RangeEnd: event.RangeEnd,
		AccessedAt: event.AccessedAt, Metadata: event.Metadata,
	})
}

// NewParentChildDataAccess records the parents portal's holiday registration
// prefill reads (#4662) in the school's data access log, in the flow's tenant
// transaction.
func NewParentChildDataAccess(writer DataAccessAppender) parentportal.ChildDataAccessLog {
	if writer == nil {
		return nil
	}
	return parentChildDataAccess{writer: writer}
}

type parentChildDataAccess struct {
	writer DataAccessAppender
}

func (a parentChildDataAccess) RecordHolidayRegistrationPrefill(ctx context.Context, access parentportal.ChildDataAccess) error {
	studentID := access.StudentID
	return a.writer.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID, ActorRole: "guardian",
		ResourceType: auditModels.ResourceTypeHolidayPrefill, StudentID: &studentID,
		RangeStart: access.AccessedAt, RangeEnd: access.AccessedAt, AccessedAt: access.AccessedAt,
		Metadata: map[string]any{"organization_id": access.OrganizationID},
	})
}

// NewHolidayParticipantAccessLog records the hosting school's reads of a
// holiday week's participant list (#4665, ADR 0116) in its own data access
// log, in the route's tenant transaction. The entry spans the week.
func NewHolidayParticipantAccessLog(writer DataAccessAppender) enrollmentOwner.HolidayParticipantAccessLog {
	if writer == nil {
		return nil
	}
	return holidayParticipantAccessLog{writer: writer}
}

type holidayParticipantAccessLog struct {
	writer DataAccessAppender
}

func (a holidayParticipantAccessLog) RecordHolidayParticipantAccess(ctx context.Context, access enrollmentOwner.HolidayParticipantAccess) error {
	resourceType := auditModels.ResourceTypeHolidayParticipants
	metadata := map[string]any{"week_id": access.WeekID, "participants": access.Participants}
	if access.HealthParticipants > 0 {
		// The list showed health information of these children (#4722).
		metadata["health_participants"] = access.HealthParticipants
	}
	if access.Format != "" {
		resourceType = auditModels.ResourceTypeHolidayParticipantsExport
		metadata["format"] = access.Format
	}
	return a.writer.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID, ActorRole: access.ActorRole, ResourceType: resourceType,
		RangeStart: access.StartsOn.BerlinMidnight(), RangeEnd: access.EndsOn.EndOfDay(), AccessedAt: time.Now(),
		Metadata: metadata,
	})
}

// RecordHolidayStudentHealthAccess records a read of what the holiday
// registrations give as health information for a child of the hosting school
// (#4722). The entry names the child and spans the weeks.
func (a holidayParticipantAccessLog) RecordHolidayStudentHealthAccess(ctx context.Context, access enrollmentOwner.HolidayStudentHealthAccess) error {
	studentID := access.StudentID
	return a.writer.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID, ActorRole: access.ActorRole, ResourceType: auditModels.ResourceTypeHolidayStudentHealth,
		StudentID: &studentID, RangeStart: access.StartsOn.BerlinMidnight(), RangeEnd: access.EndsOn.EndOfDay(), AccessedAt: time.Now(),
		Metadata: map[string]any{"week_ids": access.WeekIDs},
	})
}

// NewHolidayGuestAccessLog records the hosting school's transfers of holiday
// guests and the reads of a guest's sheet (#4666, ADR 0120) in its own data
// access log, in the caller's tenant transaction. The entry spans the week.
func NewHolidayGuestAccessLog(writer DataAccessAppender) enrollmentOwner.HolidayGuestAccessLog {
	if writer == nil {
		return nil
	}
	return holidayGuestAccessLog{writer: writer}
}

type holidayGuestAccessLog struct {
	writer DataAccessAppender
}

func (a holidayGuestAccessLog) RecordHolidayGuestAccess(ctx context.Context, access enrollmentOwner.HolidayGuestAccess) error {
	entry := &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID, ActorRole: access.ActorRole,
		RangeStart: access.StartsOn.BerlinMidnight(), RangeEnd: access.EndsOn.EndOfDay(), AccessedAt: time.Now(),
		Metadata: map[string]any{"week_id": access.WeekID},
	}
	switch access.Kind {
	case enrollmentOwner.HolidayGuestAccessSheet:
		studentID := access.StudentID
		entry.ResourceType, entry.StudentID = auditModels.ResourceTypeHolidayGuestSheet, &studentID
	default:
		entry.ResourceType = auditModels.ResourceTypeHolidayGuestTransfer
		entry.Metadata["guests"] = access.Guests
		entry.Metadata["automatic"] = access.Automatic
	}
	return a.writer.Create(ctx, entry)
}
