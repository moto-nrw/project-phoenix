package api

import (
	"context"
	"fmt"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	presenceProjection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The reads below join the Timetable plan with the Student Presence
// execution and attendance (#2762) through the tenant-safe presence legacy
// view; the root binds them to the ports of the retained services
// (ADR 0058).

// presenceProjectionReads runs the presence legacy view's reads in the
// caller's ambient transaction for the caller's tenant.
type presenceProjectionReads struct{ db transactionruntime.Database }

func newPresenceProjectionReads(db transactionruntime.Database) presenceProjectionReads {
	if db.Pool() == nil {
		panic("presence projection reads: database is required")
	}
	return presenceProjectionReads{db: db}
}

// tenant returns the caller's tenant, which every read requires.
func (presenceProjectionReads) tenant(ctx context.Context) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("presence projection: tenant is required: %w", err)
	}
	return tenantID.Int64(), nil
}

// openParticipant is a participant whose block presence is still open.
type openParticipant struct {
	ParticipantID int64
	StudentID     int64
}

// openParticipants lists the participants of the students with an open
// block presence, ordered by participant id.
func (r presenceProjectionReads) openParticipants(ctx context.Context, studentIDs []int64) ([]openParticipant, error) {
	tenantID, err := r.tenant(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceProjection.Reader{}.ListOpenParticipants(ctx, carePlanCompose.TenantAmbientDatabase(r.db.Pool())(ctx), tenantID, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]openParticipant, 0, len(rows))
	for _, row := range rows {
		result = append(result, openParticipant(row))
	}
	return result, nil
}

// latestAttendedBlockDate returns the ISO day of the last block the child
// was checked in to, or nil.
func (r presenceProjectionReads) latestAttendedBlockDate(ctx context.Context, studentID int64) (*string, error) {
	tenantID, err := r.tenant(ctx)
	if err != nil {
		return nil, err
	}
	day, err := presenceProjection.Reader{}.LatestAttendedBlockDate(ctx, carePlanCompose.TenantAmbientDatabase(r.db.Pool())(ctx), tenantID, studentID)
	if err != nil || day == nil {
		return nil, err
	}
	text := day.String()
	return &text, nil
}

// courseStatistics serves the Statistik report's course inputs.
type courseStatistics struct{ reads presenceProjectionReads }

func NewCourseStatistics(db transactionruntime.Database) courseStatistics {
	return courseStatistics{reads: newPresenceProjectionReads(db)}
}

func (c courseStatistics) CourseInstances(ctx context.Context, from, to, today timezone.Date) ([]presenceCompose.CourseInstance, error) {
	tenantID, err := c.reads.tenant(ctx)
	if err != nil {
		return nil, err
	}
	values, err := presenceProjection.Reader{}.CourseInstances(ctx, carePlanCompose.TenantAmbientDatabase(c.reads.db.Pool())(ctx), tenantID, from, to, today)
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.CourseInstance, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.CourseInstance(value))
	}
	return result, nil
}

func (c courseStatistics) CourseParticipation(ctx context.Context, from, to, today timezone.Date) ([]presenceCompose.CourseParticipation, error) {
	tenantID, err := c.reads.tenant(ctx)
	if err != nil {
		return nil, err
	}
	values, err := presenceProjection.Reader{}.CourseParticipation(ctx, carePlanCompose.TenantAmbientDatabase(c.reads.db.Pool())(ctx), tenantID, from, to, today)
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.CourseParticipation, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.CourseParticipation(value))
	}
	return result, nil
}
