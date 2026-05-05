import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { assertLocalBackendUrl } from "./safeguard";

/**
 * Path to the seeder-emitted state file. The file is written by the Go
 * seeder into the backend's working directory, which is mounted at
 * `./backend:/app` in docker compose, so on the host it lives at
 * `<repo-root>/backend/.seed-state.json`. Tests are run from the
 * `frontend/` directory, so we resolve up two levels.
 */
const SEED_STATE_PATH = resolve(
  process.cwd(),
  "..",
  "backend",
  ".seed-state.json",
);

interface SeedState {
  devices: Record<string, { api_key: string; name: string }>;
  device_pin: string;
}

let cached: SeedState | undefined;

function loadSeedState(): SeedState {
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
 * Backend HTTP base URL. Devices in production talk directly to the API
 * server, not through the Next.js proxy, so we use the same address the
 * seeder uses. Defaults to the isolated E2E backend (`server-e2e` on
 * :8081), brought up by `scripts/seed-e2e.sh`. Can be overridden via
 * `E2E_BACKEND_URL` for CI/containerised runs, but only to another local
 * URL — `assertLocalBackendUrl` below refuses anything else.
 */
export const BACKEND_URL =
  process.env.E2E_BACKEND_URL ?? "http://localhost:8081";

// Refuse module load if BACKEND_URL points anywhere non-local. Throwing
// here means tests fail before any HTTP call goes out — never possible
// to leak E2E credentials into a real environment.
assertLocalBackendUrl(BACKEND_URL);

/**
 * First seeded device's API key. The seeder names devices
 * `demo-device-001` … `demo-device-010`; we pick the first because the
 * order is stable across seeds.
 */
export function getDeviceApiKey(): string {
  const state = loadSeedState();
  const first = state.devices["demo-device-001"];
  if (!first?.api_key) {
    throw new Error(
      "demo-device-001 missing in seed state — re-run scripts/seed-e2e.sh",
    );
  }
  return first.api_key;
}

/**
 * Tenant-wide OGS device PIN. Set via the seeder; defaults to "1234".
 */
export function getDevicePIN(): string {
  return loadSeedState().device_pin;
}

export const IOT_HEADERS = {
  apiKey(key: string): Record<string, string> {
    return { Authorization: `Bearer ${key}` };
  },
  apiKeyAndPin(key: string, pin: string): Record<string, string> {
    return { Authorization: `Bearer ${key}`, "X-Staff-PIN": pin };
  },
};
