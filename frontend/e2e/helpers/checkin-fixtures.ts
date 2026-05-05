import { request as apiRequest } from "@playwright/test";
import { BACKEND_URL, IOT_HEADERS } from "./iot";
import {
  getActivityId,
  getAdminStaffId,
  getDeviceApiKey,
  getDevicePIN,
  getRoomId,
  getStudentByIndex,
} from "./seed-state";

/**
 * Stable hex RFID tag we assign to the test student. Backend stores RFID
 * card IDs as hex (case-insensitive); anything outside `[0-9a-fA-F]` is
 * rejected with "invalid RFID card ID format".
 */
const RFID_TAG_TEST_STUDENT = "E2EFE110001";

/**
 * Names of the seeded entities we want to use. We resolve them through
 * the seed-state lookups (see helpers/seed-state.ts) rather than hardcoding
 * IDs — the project's hermetic-tests rule (backend/CLAUDE.md) forbids
 * `int64(1)` because seeder reorderings cause "no rows in result set".
 */
const TEST_ROOM_NAME = "OGS-Raum 1";
const TEST_ACTIVITY_NAME = "Hausaufgaben";
const TEST_ADMIN_EMAIL = "demo1@mail.de";

export interface CheckinScenario {
  rfidTag: string;
  studentId: number;
  studentFirstName: string;
  studentLastName: string;
  roomId: number;
}

/**
 * Bootstraps everything the IoT check-in flow needs.
 *
 * For a successful kiosk-side check-in to write `active.attendance`, the
 * pipeline (`createAttendanceRecord` → `resolveStaffIDForAttendance` →
 * `getDeviceSupervisorID`) requires an active group that's BOTH bound to
 * the device (`device_id` set) AND has a row in `active.group_supervisors`.
 * The CRUD endpoint `POST /api/active/groups` exposes neither field — the
 * kiosk-only `POST /api/iot/session/start` is the only way to create
 * exactly that shape, so that's what we use.
 *
 * Steps:
 *   1. Resolve the test student / room / activity / supervisor by name
 *      from .seed-state.json (no hardcoded IDs).
 *   2. Start an activity session as the device, with the resolved
 *      supervisor. The endpoint takes care of `device_id` + supervisor
 *      wiring.
 *   3. Assign a known hex RFID tag to the test student — the seeder does
 *      not populate `users.rfid_cards` for any student.
 *
 * Both API steps tolerate prior runs (using `force: true` for the session,
 * idempotent re-assign for the RFID tag), so the scenario can be set up
 * repeatedly without `migrate reset`.
 */
export async function setupCheckinScenario(): Promise<CheckinScenario> {
  const student = getStudentByIndex(0);
  const roomId = getRoomId(TEST_ROOM_NAME);
  const activityId = getActivityId(TEST_ACTIVITY_NAME);
  const supervisorStaffId = getAdminStaffId(TEST_ADMIN_EMAIL);

  const ctx = await apiRequest.newContext();
  try {
    await ensureActivitySessionForDevice(ctx, {
      activityId,
      roomId,
      supervisorStaffId,
    });
    await ensureRfidAssignment(ctx, student.id);
    return {
      rfidTag: RFID_TAG_TEST_STUDENT,
      studentId: student.id,
      studentFirstName: student.first_name,
      studentLastName: student.last_name,
      roomId,
    };
  } finally {
    await ctx.dispose();
  }
}

type ApiContext = Awaited<ReturnType<typeof apiRequest.newContext>>;

interface SessionParams {
  activityId: number;
  roomId: number;
  supervisorStaffId: number;
}

async function ensureActivitySessionForDevice(
  ctx: ApiContext,
  params: SessionParams,
): Promise<void> {
  const headers = {
    ...IOT_HEADERS.apiKeyAndPin(getDeviceApiKey(), getDevicePIN()),
    "Content-Type": "application/json",
  };

  // `force: true` overrides any pre-existing session for this device, so the
  // setup is idempotent. Without force, a second run would 409 with
  // "device already has an active session".
  const res = await ctx.post(`${BACKEND_URL}/api/iot/session/start`, {
    headers,
    data: {
      activity_id: params.activityId,
      room_id: params.roomId,
      supervisor_ids: [params.supervisorStaffId],
      force: true,
    },
    failOnStatusCode: false,
  });

  if (!res.ok()) {
    throw new Error(
      `start activity session failed (${res.status()}): ${await res.text()}`,
    );
  }
}

async function ensureRfidAssignment(
  ctx: ApiContext,
  studentId: number,
): Promise<void> {
  // The handler (backend/api/students/rfid_handlers.go:assignRFIDTag) is
  // idempotent end-to-end: re-assigning the same tag, or moving a tag
  // between students, both return 200. Anything outside 2xx is a real
  // failure (401 auth broken, 404 wrong student id, 400 bad payload) that
  // we want surfaced — masking 4xx hid exactly those cases and made the
  // checkin test fail later with a misleading "no student matches RFID".
  const res = await ctx.post(`${BACKEND_URL}/api/students/${studentId}/rfid`, {
    headers: {
      ...IOT_HEADERS.apiKeyAndPin(getDeviceApiKey(), getDevicePIN()),
      "Content-Type": "application/json",
    },
    data: { rfid_tag: RFID_TAG_TEST_STUDENT },
    failOnStatusCode: false,
  });

  if (!res.ok()) {
    throw new Error(
      `assign RFID tag failed (${res.status()}): ${await res.text()}`,
    );
  }
}
