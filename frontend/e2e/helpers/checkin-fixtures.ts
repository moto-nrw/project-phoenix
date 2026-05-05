import { request as apiRequest } from "@playwright/test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { BACKEND_URL, IOT_HEADERS, getDeviceApiKey, getDevicePIN } from "./iot";

/**
 * Stable hex RFID tag we assign to Felix Schneider for E2E. The backend
 * stores RFID card IDs as hex (case-insensitive), so anything outside
 * `[0-9a-fA-F]` would be rejected with "invalid RFID card ID format".
 */
const RFID_TAG_FELIX = "E2EFE110001";

/** Education group "sternengruppe" — see backend/.seed-state.json lookups.groups. */
const TEST_ACTIVITY_ID = 1;

/** Room "OGS-Raum 1" — see backend/.seed-state.json lookups.rooms. */
const TEST_ROOM_ID = 1;

/** First seeded student — Felix Schneider, see backend/seed/api/data.go. */
const TEST_STUDENT_ID = 1;

const SEED_STATE_PATH = resolve(
  process.cwd(),
  "..",
  "backend",
  ".seed-state.json",
);

interface SeedState {
  accounts: { admin: Array<{ email: string; staff_id: number }> };
}

let cachedAdminStaffID: number | undefined;

/** Reads demo1's staff_id from the seeder state file. */
function getDemo1StaffID(): number {
  if (cachedAdminStaffID !== undefined) return cachedAdminStaffID;
  const raw = readFileSync(SEED_STATE_PATH, "utf-8");
  const state = JSON.parse(raw) as SeedState;
  const demo1 = state.accounts.admin.find((a) => a.email === "demo1@mail.de");
  if (!demo1?.staff_id) {
    throw new Error(
      `demo1@mail.de not found in ${SEED_STATE_PATH}.accounts.admin — re-seed first`,
    );
  }
  cachedAdminStaffID = demo1.staff_id;
  return demo1.staff_id;
}

export interface CheckinScenario {
  rfidTag: string;
  studentId: number;
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
 *   1. Start an activity session as the device, with demo1 as supervisor.
 *      The endpoint takes care of `device_id` + supervisor wiring.
 *   2. Assign a known hex RFID tag to Felix Schneider — the seeder does
 *      not populate `users.rfid_cards` for any student.
 *
 * Both steps tolerate prior runs (4xx → assume already-set), so the
 * scenario can be set up repeatedly without `migrate reset`.
 */
export async function setupCheckinScenario(): Promise<CheckinScenario> {
  const ctx = await apiRequest.newContext();
  try {
    await ensureActivitySessionForDevice(ctx);
    await ensureRfidAssignment(ctx);
    return {
      rfidTag: RFID_TAG_FELIX,
      studentId: TEST_STUDENT_ID,
      roomId: TEST_ROOM_ID,
    };
  } finally {
    await ctx.dispose();
  }
}

type ApiContext = Awaited<ReturnType<typeof apiRequest.newContext>>;

async function ensureActivitySessionForDevice(ctx: ApiContext): Promise<void> {
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
      activity_id: TEST_ACTIVITY_ID,
      room_id: TEST_ROOM_ID,
      supervisor_ids: [getDemo1StaffID()],
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

async function ensureRfidAssignment(ctx: ApiContext): Promise<void> {
  // The handler (backend/api/students/rfid_handlers.go:assignRFIDTag) is
  // idempotent end-to-end: re-assigning the same tag, or moving a tag
  // between students, both return 200. Anything outside 2xx is a real
  // failure (401 auth broken, 404 wrong student id, 400 bad payload) that
  // we want surfaced — masking 4xx hid exactly those cases and made the
  // checkin test fail later with a misleading "no student matches RFID".
  const res = await ctx.post(
    `${BACKEND_URL}/api/students/${TEST_STUDENT_ID}/rfid`,
    {
      headers: {
        ...IOT_HEADERS.apiKeyAndPin(getDeviceApiKey(), getDevicePIN()),
        "Content-Type": "application/json",
      },
      data: { rfid_tag: RFID_TAG_FELIX },
      failOnStatusCode: false,
    },
  );

  if (!res.ok()) {
    throw new Error(
      `assign RFID tag failed (${res.status()}): ${await res.text()}`,
    );
  }
}
