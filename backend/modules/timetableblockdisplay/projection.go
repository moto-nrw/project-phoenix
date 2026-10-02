// Package timetableblockdisplay contains the tenant-safe display projection
// for scheduled timetable blocks.
package timetableblockdisplay

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

// ErrInvalidTenantID reports a missing or non-positive projection tenant.
var ErrInvalidTenantID = errors.New("timetable block display: tenant ID must be positive")

type row struct {
	InstanceID             int64   `bun:"instance_id"`
	RoomName               string  `bun:"room_name"`
	ActivityType           string  `bun:"activity_type"`
	RequiredStaff          *int    `bun:"required_staff"`
	PlanningTrackID        *int64  `bun:"planning_track_id"`
	PlanningTrackName      string  `bun:"planning_track_name"`
	PlanningTrackColor     string  `bun:"planning_track_color"`
	PlanningTrackSortOrder *int    `bun:"planning_track_sort_order"`
	SeriesNotes            *string `bun:"series_notes"`
	SourceCareOfferingIDs  []int64 `bun:"source_care_offering_ids,type:jsonb,nullzero"`
	MaxParticipants        int     `bun:"max_participants"`
	GroupName              string  `bun:"group_name"`
}

// List reads the room and template details the timetable renders in each
// block. Every join is scoped to tenantID, so the projection remains safe
// when a window mixes spontaneous and template-backed blocks.
func List(ctx context.Context, db bun.IDB, tenantID int64, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	result := make(map[int64]timetable.BlockDisplayMetadata, len(instanceIDs))
	if len(instanceIDs) == 0 {
		return result, nil
	}
	rows := make([]row, 0, len(instanceIDs))
	err := db.NewSelect().
		TableExpr(`schedule.activity_instances AS "instance"`).
		ColumnExpr(`"instance".id AS instance_id`).
		ColumnExpr(`COALESCE(room.name, '') AS room_name`).
		ColumnExpr(`COALESCE(template.type, 'activity') AS activity_type`).
		ColumnExpr(`template.required_staff`).
		ColumnExpr(`template.planning_track_id`).
		ColumnExpr(`COALESCE(track.name, '') AS planning_track_name`).
		ColumnExpr(`COALESCE(track.color, '') AS planning_track_color`).
		ColumnExpr(`track.sort_order AS planning_track_sort_order`).
		ColumnExpr(`template.notes AS series_notes`).
		ColumnExpr(`COALESCE(template.source_care_offering_ids, '[]'::jsonb) AS source_care_offering_ids`).
		ColumnExpr(`COALESCE(template.max_participants, 0) AS max_participants`).
		ColumnExpr(`COALESCE(education_group.name, '') AS group_name`).
		Join(`LEFT JOIN facilities.rooms AS room ON room.id = "instance".room_id AND room.tenant_id = "instance".tenant_id`).
		Join(`LEFT JOIN activities.groups AS template ON template.id = "instance".activity_group_id AND template.tenant_id = "instance".tenant_id`).
		Join(`LEFT JOIN schedule.planning_tracks AS track ON track.id = template.planning_track_id AND track.tenant_id = "instance".tenant_id`).
		Join(`LEFT JOIN education.groups AS education_group ON education_group.id = template.education_group_id AND education_group.tenant_id = "instance".tenant_id`).
		Where(`"instance".tenant_id = ?`, tenantID).
		Where(`"instance".id IN (?)`, bun.List(instanceIDs)).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("timetable block display: list: %w", err)
	}
	for _, value := range rows {
		result[value.InstanceID] = timetable.BlockDisplayMetadata{
			RoomName:               value.RoomName,
			ActivityType:           value.ActivityType,
			RequiredStaff:          value.RequiredStaff,
			PlanningTrackID:        value.PlanningTrackID,
			PlanningTrackName:      value.PlanningTrackName,
			PlanningTrackColor:     value.PlanningTrackColor,
			PlanningTrackSortOrder: value.PlanningTrackSortOrder,
			SeriesNotes:            value.SeriesNotes,
			SourceCareOfferingIDs:  append([]int64(nil), value.SourceCareOfferingIDs...),
			ParticipantLimit:       timetable.ParticipantLimitPtr(value.MaxParticipants),
			GroupName:              value.GroupName,
		}
	}
	return result, nil
}
