package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const roomsArchiveVersion = "20261113130704"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomsArchiveVersion,
		Description: "Räume archivieren statt löschen; Verweise auf Räume nie mehr still leeren (#6845)",
		DependsOn: []string{
			compositeFKsVersion,                  // the room keys of groups and activity groups (1.15.2)
			iotDevicesRoomIDVersion,              // the room key of devices (1.15.28)
			activityInstancesCompositeFKsVersion, // the room keys of instances, staff, children and exceptions (1.15.51)
			addIsSystemFlagsVersion,              // facilities.rooms.is_system (1.15.168)
		},
	})
	Migrations.MustRegister(roomsArchiveUp, roomsArchiveDown)
}

// roomsArchiveUpSQL keeps a removed room that history still names (#6845).
//
//   - archived_at marks such a room; lists and name lookups skip it, so its
//     name and colour are free again (partial unique indexes).
//   - Every key to facilities.rooms becomes NO ACTION: deleting a referenced
//     room fails instead of silently emptying the planned room of activities,
//     the room of education groups, devices, staff and child rows and
//     exceptions. Facilities then archives the room. School erasure deletes
//     those rows before the rooms, so it is unaffected.
//   - The toilet and yard rooms carry is_system whatever their origin; the
//     protections read the flag, not the name.
const roomsArchiveUpSQL = `
ALTER TABLE facilities.rooms ADD COLUMN archived_at TIMESTAMPTZ;
COMMENT ON COLUMN facilities.rooms.archived_at IS
	'When the school removed the room while past blocks or sessions still named it (#6845); NULL while the room is in use.';

UPDATE facilities.rooms SET is_system = TRUE
WHERE NOT is_system AND name IN ('Schulhof', 'WC', 'Toilette');

DROP INDEX facilities.idx_rooms_tenant_name;
CREATE UNIQUE INDEX idx_rooms_tenant_name ON facilities.rooms (tenant_id, name) WHERE archived_at IS NULL;
DROP INDEX facilities.uniq_facilities_rooms_tenant_color;
CREATE UNIQUE INDEX uniq_facilities_rooms_tenant_color ON facilities.rooms (tenant_id, lower(color))
	WHERE color IS NOT NULL AND archived_at IS NULL;

ALTER TABLE activities.groups DROP CONSTRAINT fk_activity_groups_room_tenant,
	ADD CONSTRAINT fk_activity_groups_room_tenant FOREIGN KEY (tenant_id, planned_room_id)
	REFERENCES facilities.rooms (tenant_id, id) NOT VALID;
ALTER TABLE education.groups DROP CONSTRAINT fk_groups_room_tenant,
	ADD CONSTRAINT fk_groups_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) NOT VALID;
ALTER TABLE iot.devices DROP CONSTRAINT fk_iot_devices_room_tenant,
	ADD CONSTRAINT fk_iot_devices_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) NOT VALID;
ALTER TABLE schedule.activity_exceptions DROP CONSTRAINT fk_activity_exceptions_room_tenant,
	ADD CONSTRAINT fk_activity_exceptions_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) NOT VALID;
ALTER TABLE schedule.instance_staff DROP CONSTRAINT fk_instance_staff_room_tenant,
	ADD CONSTRAINT fk_instance_staff_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) NOT VALID;
ALTER TABLE schedule.instance_students DROP CONSTRAINT fk_instance_students_room_tenant,
	ADD CONSTRAINT fk_instance_students_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) NOT VALID;

ALTER TABLE activities.groups VALIDATE CONSTRAINT fk_activity_groups_room_tenant;
ALTER TABLE education.groups VALIDATE CONSTRAINT fk_groups_room_tenant;
ALTER TABLE iot.devices VALIDATE CONSTRAINT fk_iot_devices_room_tenant;
ALTER TABLE schedule.activity_exceptions VALIDATE CONSTRAINT fk_activity_exceptions_room_tenant;
ALTER TABLE schedule.instance_staff VALIDATE CONSTRAINT fk_instance_staff_room_tenant;
ALTER TABLE schedule.instance_students VALIDATE CONSTRAINT fk_instance_students_room_tenant;
`

// roomsArchiveDownSQL restores the former keys. Archived rooms stay rows;
// their names may collide with live rooms, which the down migration refuses.
const roomsArchiveDownSQL = `
ALTER TABLE activities.groups DROP CONSTRAINT fk_activity_groups_room_tenant,
	ADD CONSTRAINT fk_activity_groups_room_tenant FOREIGN KEY (tenant_id, planned_room_id)
	REFERENCES facilities.rooms (tenant_id, id) ON DELETE SET NULL;
ALTER TABLE education.groups DROP CONSTRAINT fk_groups_room_tenant,
	ADD CONSTRAINT fk_groups_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) ON DELETE SET NULL;
ALTER TABLE iot.devices DROP CONSTRAINT fk_iot_devices_room_tenant,
	ADD CONSTRAINT fk_iot_devices_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) ON DELETE SET NULL (room_id);
ALTER TABLE schedule.activity_exceptions DROP CONSTRAINT fk_activity_exceptions_room_tenant,
	ADD CONSTRAINT fk_activity_exceptions_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) ON DELETE SET NULL (room_id);
ALTER TABLE schedule.instance_staff DROP CONSTRAINT fk_instance_staff_room_tenant,
	ADD CONSTRAINT fk_instance_staff_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) ON DELETE SET NULL (room_id);
ALTER TABLE schedule.instance_students DROP CONSTRAINT fk_instance_students_room_tenant,
	ADD CONSTRAINT fk_instance_students_room_tenant FOREIGN KEY (tenant_id, room_id)
	REFERENCES facilities.rooms (tenant_id, id) ON DELETE SET NULL (room_id);

DROP INDEX facilities.uniq_facilities_rooms_tenant_color;
CREATE UNIQUE INDEX uniq_facilities_rooms_tenant_color ON facilities.rooms (tenant_id, lower(color)) WHERE color IS NOT NULL;
DROP INDEX facilities.idx_rooms_tenant_name;
CREATE UNIQUE INDEX idx_rooms_tenant_name ON facilities.rooms (tenant_id, name);
ALTER TABLE facilities.rooms DROP COLUMN archived_at;
`

func roomsArchiveUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, roomsArchiveUpSQL); err != nil {
		return fmt.Errorf("archive rooms instead of deleting them: %w", err)
	}
	return nil
}

// roomsArchiveFingerprint is the incoming-FK fingerprint of the school
// erasure plan with the NO ACTION room keys; the repeatable preflight checks
// it. roomsArchiveBaseFingerprint is the one before (1.15.960).
const (
	roomsArchiveFingerprint     = "61af89cd3a78c8f8f87ae712e7fc3b095a329ef36b0e6e67ca1953f1da635cb4"
	roomsArchiveBaseFingerprint = pickupChangeTeamNoticesFingerprint
)

// roomsArchiveDown restores the former keys and points the installed erasure
// preflight back at their fingerprint, so a school erasure after the
// rollback still passes its signature check.
func roomsArchiveDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, roomsArchiveDownSQL); err != nil {
			return fmt.Errorf("restore room deletion keys: %w", err)
		}
		var definition string
		if err := tx.NewRaw(`SELECT pg_get_functiondef('platform.school_erasure_preflight(bigint,bigint)'::regprocedure)`).Scan(ctx, &definition); err != nil {
			return fmt.Errorf("read the school erasure preflight: %w", err)
		}
		if strings.Count(definition, "'"+roomsArchiveFingerprint+"'") != 1 {
			return nil // an older preflight is installed; it already checks the restored keys
		}
		restored := strings.Replace(definition, "'"+roomsArchiveFingerprint+"'", "'"+roomsArchiveBaseFingerprint+"'", 1)
		if err := installDerivedSchoolErasureFunctions(ctx, tx, []string{restored}); err != nil {
			return fmt.Errorf("point the school erasure preflight at the restored keys: %w", err)
		}
		return nil
	})
}
