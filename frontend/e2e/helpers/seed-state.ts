import { readFileSync } from "node:fs";
import { resolve } from "node:path";

/**
 * Single source of truth for reading the deterministic seeder's output
 * (`backend/.seed-state-e2e.json`). The seeder writes this file from
 * inside the `server-e2e` container, which mounts `./backend:/app`, so
 * the host sees it at `<repo>/backend/.seed-state-e2e.json`. Tests run
 * from `frontend/`, so we resolve up one level.
 *
 * Why the `-e2e` suffix: both the dev `server` and `server-e2e`
 * containers mount the same `./backend` directory. Without a separate
 * filename, `docker compose run server ./main seed` (dev) and
 * `./scripts/seed-e2e.sh` (E2E) would both write to `.seed-state.json`
 * and silently clobber each other. Keep this filename in sync with the
 * `--state-path` flag passed in `scripts/seed-e2e.sh`.
 *
 * Why a dedicated module: the backend CLAUDE.md hermetic rule forbids
 * hardcoded IDs (`int64(1)`-`int64(9)`) because seeder reorderings cause
 * "sql: no rows in result set" failures. Anything in the test suite that
 * needs a specific seeded entity MUST go through one of the lookup
 * helpers below — never `1`, never `"Felix Schneider"` literally — so a
 * future seeder change never silently moves the tested entity.
 *
 * The state struct is stable per `CurrentSeedStateVersion` in
 * `backend/seed/api/state.go`. If that bumps, this file must be updated
 * to track new fields.
 */

const SEED_STATE_PATH = resolve(
  process.cwd(),
  "..",
  "backend",
  ".seed-state-e2e.json",
);

interface SeedDevice {
  api_key: string;
  name: string;
}

interface SeedStudent {
  id: number;
  first_name: string;
  last_name: string;
  group_key: string;
  class: string;
}

interface SeedAccount {
  email: string;
  password: string;
  pin: string;
  name: string;
  staff_id: number;
  teacher_id?: number;
  group?: string;
}

interface SeedStateLookups {
  rooms: Record<string, number>;
  activities: Record<string, number>;
  groups: Record<string, number>;
}

export interface SeedState {
  device_pin: string;
  devices: Record<string, SeedDevice>;
  students: SeedStudent[];
  accounts: { admin: SeedAccount[]; betreuer: SeedAccount[] };
  lookups: SeedStateLookups;
  // Top-level legacy mirrors of `lookups.*`. The Go seeder writes both
  // shapes in `Normalize()` for backwards compatibility, but we read
  // through `lookups.*` exclusively to avoid drifting between them.
}

let cached: SeedState | undefined;

export function loadSeedState(): SeedState {
  if (cached) return cached;
  try {
    const raw = readFileSync(SEED_STATE_PATH, "utf-8");
    cached = JSON.parse(raw) as SeedState;
    return cached;
  } catch (err) {
    throw new Error(
      `Could not read ${SEED_STATE_PATH}. Run \`./scripts/seed-e2e.sh\` first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

/**
 * Returns the seeded student at `index`. Index 0 is the first student in
 * `DemoStudents` (currently Felix Schneider, but tests must not assume the
 * name). Throws if the seeder produced fewer students than expected — that
 * is always a real failure (incomplete seed), not "test data missing".
 */
export function getStudentByIndex(index: number): SeedStudent {
  const students = loadSeedState().students;
  const student = students[index];
  if (!student) {
    throw new Error(
      `seed-state has only ${students.length} students; index ${index} requested. ` +
        `Re-run scripts/seed-e2e.sh.`,
    );
  }
  return student;
}

/**
 * Returns the room id for a known room name (e.g. "OGS-Raum 1"). Names
 * come from `backend/seed/api/data.go:DemoRooms`. Throws on miss because
 * a missing room means the seed is incomplete; a misleading "not found
 * in API list" downstream error is harder to diagnose.
 */
export function getRoomId(name: string): number {
  const id = loadSeedState().lookups.rooms[name];
  if (!id) {
    throw new Error(
      `Room "${name}" missing from seed-state.lookups.rooms. ` +
        `Known rooms: ${Object.keys(loadSeedState().lookups.rooms).join(", ")}`,
    );
  }
  return id;
}

/**
 * Returns the activity id for a known activity name (e.g. "Hausaufgaben").
 * Names come from `backend/seed/api/data.go:DemoActivities`.
 */
export function getActivityId(name: string): number {
  const id = loadSeedState().lookups.activities[name];
  if (!id) {
    throw new Error(
      `Activity "${name}" missing from seed-state.lookups.activities. ` +
        `Known activities: ${Object.keys(loadSeedState().lookups.activities).join(", ")}`,
    );
  }
  return id;
}

/**
 * Returns the group id for a known group key (e.g. "sternengruppe").
 * Keys come from `backend/seed/api/data.go:DemoStudents.GroupKey`.
 */
export function getGroupId(key: string): number {
  const id = loadSeedState().lookups.groups[key];
  if (!id) {
    throw new Error(
      `Group "${key}" missing from seed-state.lookups.groups. ` +
        `Known groups: ${Object.keys(loadSeedState().lookups.groups).join(", ")}`,
    );
  }
  return id;
}

/**
 * Returns N seeded students starting at `start`. Used for UI assertions
 * that need a couple of students whose names are guaranteed to exist on
 * the seeded /database/students list, without picking specific indices
 * scattered through several specs.
 */
export function getStudents(start: number, count: number): SeedStudent[] {
  const out: SeedStudent[] = [];
  for (let i = 0; i < count; i++) {
    out.push(getStudentByIndex(start + i));
  }
  return out;
}

/**
 * Returns two seeded students with distinct first AND last names. This is
 * the contract the search-filter UI test depends on: typing student A's
 * first name must filter student B out of the list. With duplicate first
 * names somewhere in the seed (cross-group), an `index 0/1` pick could
 * silently break that assumption — so we walk the seeded list until we
 * find the first pair where both names differ.
 *
 * Throws if no such pair exists in the seed (would mean the seed has
 * fewer than two distinct students, which is itself a real bug).
 */
export function getTwoDistinctStudents(): [SeedStudent, SeedStudent] {
  const students = loadSeedState().students;
  const first = students[0];
  if (!first) {
    throw new Error("seed-state has no students — re-run scripts/seed-e2e.sh");
  }
  for (let i = 1; i < students.length; i++) {
    const candidate = students[i];
    if (
      candidate &&
      candidate.first_name !== first.first_name &&
      candidate.last_name !== first.last_name
    ) {
      return [first, candidate];
    }
  }
  throw new Error(
    "seed-state has no two students with distinct first AND last names — " +
      "re-run scripts/seed-e2e.sh or update the seeder if this is intentional",
  );
}

/**
 * Returns a tuple of the first two seeded groups' display names so UI
 * tests can assert the group list renders without hardcoding which groups
 * are first. Display name = key with first letter capitalised (matches
 * the backend render path in `models/education/group.go:DisplayName`).
 *
 * Returns a fixed-arity tuple rather than `string[]` because the consumer
 * destructures two values and `noUncheckedIndexedAccess` would otherwise
 * make each element `string | undefined`.
 */
export function getFirstTwoGroupDisplayNames(): [string, string] {
  const keys = Object.keys(loadSeedState().lookups.groups);
  const first = keys[0];
  const second = keys[1];
  if (!first || !second) {
    throw new Error(
      `seed-state has only ${keys.length} groups; need at least 2. ` +
        `Re-run scripts/seed-e2e.sh.`,
    );
  }
  return [displayNameForGroupKey(first), displayNameForGroupKey(second)];
}

function displayNameForGroupKey(key: string): string {
  return key.charAt(0).toUpperCase() + key.slice(1);
}

/**
 * Reads the staff_id for a known admin account email. Used by checkin
 * fixtures to wire `demo1@mail.de` as the supervisor for the test
 * activity session.
 */
export function getAdminStaffId(email: string): number {
  const account = loadSeedState().accounts.admin.find((a) => a.email === email);
  if (!account?.staff_id) {
    throw new Error(
      `${email} not found in seed-state.accounts.admin — re-run scripts/seed-e2e.sh`,
    );
  }
  return account.staff_id;
}

/** First seeded device's API key (`demo-device-001`). */
export function getDeviceApiKey(): string {
  const device = loadSeedState().devices["demo-device-001"];
  if (!device?.api_key) {
    throw new Error(
      "demo-device-001 missing in seed state — re-run scripts/seed-e2e.sh",
    );
  }
  return device.api_key;
}

/** Tenant-wide OGS device PIN. */
export function getDevicePIN(): string {
  const pin = loadSeedState().device_pin;
  if (!pin) {
    throw new Error(
      "device_pin missing in seed state — re-run scripts/seed-e2e.sh",
    );
  }
  return pin;
}

/**
 * Cache reset hook — only used by tests of seed-state itself, not by
 * production specs. Exported because Vitest-style modules can't reach
 * the closure variable directly.
 */
export function _resetCacheForTesting(): void {
  cached = undefined;
}
