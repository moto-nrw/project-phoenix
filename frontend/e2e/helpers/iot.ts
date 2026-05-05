import { assertLocalBackendUrl } from "./safeguard";

/**
 * IoT-side test helpers. The seed-state file lookups themselves live in
 * `seed-state.ts` so all `.seed-state.json` access goes through one
 * module — re-exported here so existing imports keep working.
 */
export { getDeviceApiKey, getDevicePIN } from "./seed-state";

/**
 * Backend HTTP base URL. Devices in production talk directly to the API
 * server, not through the Next.js proxy, so we use the same address the
 * seeder uses. Required, no fallback (project "No fallbacks" rule):
 * `scripts/seed-e2e.sh` exports `E2E_BACKEND_URL` for local runs and CI
 * sets it explicitly. `assertLocalBackendUrl` below additionally refuses
 * anything that isn't loopback.
 */
const RAW_BACKEND_URL = process.env.E2E_BACKEND_URL;
if (!RAW_BACKEND_URL) {
  throw new Error(
    "E2E_BACKEND_URL is not set. Run via scripts/seed-e2e.sh (which exports " +
      "it for you) or export E2E_BACKEND_URL=http://localhost:8081 manually.",
  );
}
export const BACKEND_URL = RAW_BACKEND_URL;

// Refuse module load if BACKEND_URL points anywhere non-local. Throwing
// here means tests fail before any HTTP call goes out — never possible
// to leak E2E credentials into a real environment.
assertLocalBackendUrl(BACKEND_URL);

export const IOT_HEADERS = {
  apiKey(key: string): Record<string, string> {
    return { Authorization: `Bearer ${key}` };
  },
  apiKeyAndPin(key: string, pin: string): Record<string, string> {
    return { Authorization: `Bearer ${key}`, "X-Staff-PIN": pin };
  },
};
