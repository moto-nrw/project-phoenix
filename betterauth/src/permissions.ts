import { createAccessControl } from "better-auth/plugins/access";

/**
 * Permission statement for Project Phoenix multi-tenant system.
 *
 * Resources and their allowed actions:
 * - student: Student records (CRUD)
 * - group: Education groups (CRUD + assign students)
 * - room: Physical rooms/spaces (CRUD)
 * - attendance: Check-in/out records
 * - location: GDPR SENSITIVE - real-time location data (which room, check-in times)
 * - staff: Staff members (CRUD + invite)
 * - ogs: OGS settings and configuration
 * - config: System configuration settings (admin only)
 * - schedule: Timeframes, dateframes, recurrence rules
 * - feedback: Student emotional check-ins
 * - substitution: Group supervisor substitutions
 *
 * GDPR CRITICAL: The `location` resource contains sensitive real-time data about
 * where students are physically located. Only roles with direct operational need
 * (supervisor, ogsAdmin) may have this permission. Administrative roles
 * (bueroAdmin, traegerAdmin) who manage remotely MUST NOT have location access.
 */
export const statement = {
  student: ["read", "create", "update", "delete"],
  group: ["read", "create", "update", "delete", "assign"], // assign = add/remove students
  room: ["read", "create", "update", "delete"],
  attendance: ["read", "checkin", "checkout"],
  location: ["read"], // GDPR sensitive - only operational roles!
  staff: ["read", "create", "update", "delete", "invite"],
  ogs: ["read", "update"],
  config: ["read", "update"], // System configuration - admin only
  schedule: ["read", "create", "update", "delete"], // Timeframes and schedules
  feedback: ["read", "create", "delete"], // Student emotional check-ins
  substitution: ["read", "create", "update", "delete"], // Group substitutions
} as const;

/**
 * Access controller instance.
 * Pass this to the organization plugin to enable permission checking.
 */
export const ac = createAccessControl(statement);

/**
 * Supervisor role - Front-line staff working directly with students.
 *
 * Scope: Only their assigned groups (enforced at query level, not here)
 *
 * Permissions:
 * - Can view and update student info
 * - Can view groups (assignment handled separately)
 * - Can perform check-in/checkout operations
 * - CAN see location data (operational need)
 * - Can read schedules, create/read feedback, read substitutions
 * - Cannot: create/delete students, manage groups/rooms/staff, update OGS settings
 */
export const supervisor = ac.newRole({
  student: ["read", "update"],
  group: ["read"],
  attendance: ["read", "checkin", "checkout"],
  location: ["read"], // Operational staff needs to see where students are
  schedule: ["read"], // Can view schedules
  feedback: ["read", "create"], // Can record student feedback
  substitution: ["read"], // Can view substitutions
});

/**
 * OGS Admin role - Full administrator for a single OGS (after-school center).
 *
 * Scope: All data within their OGS
 *
 * Permissions:
 * - Full student management
 * - Full group management including student assignment
 * - Full room management
 * - Full attendance access
 * - CAN see location data (runs the OGS operationally)
 * - Staff management (cannot delete - only higher roles)
 * - OGS settings management
 * - Full config, schedule, feedback, substitution management
 */
export const ogsAdmin = ac.newRole({
  student: ["read", "create", "update", "delete"],
  group: ["read", "create", "update", "delete", "assign"],
  room: ["read", "create", "update", "delete"],
  attendance: ["read", "checkin", "checkout"],
  location: ["read"], // OGS admin runs operations, needs location visibility
  staff: ["read", "create", "update", "invite"],
  ogs: ["read", "update"],
  config: ["read", "update"], // Full config management
  schedule: ["read", "create", "update", "delete"], // Full schedule management
  feedback: ["read", "create", "delete"], // Full feedback management
  substitution: ["read", "create", "update", "delete"], // Full substitution management
});

/**
 * Buro Admin role - Office administrator managing multiple OGS remotely.
 *
 * Scope: All OGS under their Buro (office)
 *
 * GDPR CRITICAL: This role DOES NOT have location:read permission!
 * Buro admins manage from a distance and have no operational need to know
 * which room a specific student is in at any moment. This is a legal
 * requirement for GDPR compliance, not a preference.
 *
 * Permissions:
 * - Full student management
 * - Full group management (no assign - that's operational)
 * - Attendance records (can see that attendance happened)
 * - NO location data - GDPR restriction
 * - Full staff management including delete
 * - OGS settings management
 * - Full config, schedule, feedback, substitution management
 */
export const bueroAdmin = ac.newRole({
  student: ["read", "create", "update", "delete"],
  group: ["read", "create", "update", "delete"],
  attendance: ["read"],
  // location: INTENTIONALLY OMITTED - GDPR compliance!
  staff: ["read", "create", "update", "delete", "invite"],
  ogs: ["read", "update"],
  config: ["read", "update"], // Full config management
  schedule: ["read", "create", "update", "delete"], // Full schedule management
  feedback: ["read", "create", "delete"], // Full feedback management
  substitution: ["read", "create", "update", "delete"], // Full substitution management
});

/**
 * Traeger Admin role - Carrier administrator (highest level), manages all OGS.
 *
 * Scope: All OGS under their Traeger (carrier/provider)
 *
 * GDPR CRITICAL: This role DOES NOT have location:read permission!
 * Traeger admins are the highest administrative level and manage from a
 * distance. They have no operational need to track real-time student locations.
 * This is a legal requirement for GDPR compliance, not a preference.
 *
 * Permissions:
 * - Full student management
 * - Full group management (no assign - that's operational)
 * - Attendance records (can see that attendance happened)
 * - NO location data - GDPR restriction
 * - Full staff management including delete
 * - OGS settings management
 * - Full config, schedule, feedback, substitution management
 */
export const traegerAdmin = ac.newRole({
  student: ["read", "create", "update", "delete"],
  group: ["read", "create", "update", "delete"],
  attendance: ["read"],
  // location: INTENTIONALLY OMITTED - GDPR compliance!
  staff: ["read", "create", "update", "delete", "invite"],
  ogs: ["read", "update"],
  config: ["read", "update"], // Full config management
  schedule: ["read", "create", "update", "delete"], // Full schedule management
  feedback: ["read", "create", "delete"], // Full feedback management
  substitution: ["read", "create", "update", "delete"], // Full substitution management
});
