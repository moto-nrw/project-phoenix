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

export const IOT_HEADERS = {
  apiKey(key: string): Record<string, string> {
    return { Authorization: `Bearer ${key}` };
  },
  apiKeyAndPin(key: string, pin: string): Record<string, string> {
    return { Authorization: `Bearer ${key}`, "X-Staff-PIN": pin };
  },
};
