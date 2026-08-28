/**
 * PWA standalone-usage reporting (#2189): when an authenticated session runs
 * in standalone display mode (installed to the home screen), tell the backend
 * once per browser session. Fire-and-forget telemetry — every failure is
 * swallowed into a debug log, never surfaced to the user.
 */

import { createLogger } from "~/lib/logger";
import { isStandaloneApp, type PushPortal } from "~/lib/push-api";

const logger = createLogger({ component: "PwaUsageApi" });

const SESSION_REPORTED_KEY_PREFIX = "moto-pwa-usage-reported.";

function reportPath(portal: PushPortal): string {
  if (portal === "parent") return "/api/parent/me/pwa-usage";
  if (portal === "tenant") return "/api/pwa/usage";
  return "";
}

/**
 * Reports standalone usage for the current session. No-op outside standalone
 * mode and after a successful report in the same browser session (the server
 * upsert makes repeats harmless anyway — the guard just saves the request).
 */
export async function reportStandaloneUsage(
  portal: PushPortal,
  accountID: string,
  tenantID?: number,
): Promise<void> {
  if (!isStandaloneApp()) return;

  // School sessions have no PWA-usage API route. Do not send their school JWT
  // to the tenant-only endpoint, which rejects that scope.
  const path = reportPath(portal);
  if (!path) return;

  const sessionKey = `${SESSION_REPORTED_KEY_PREFIX}${portal}.${accountID}.${tenantID ?? "parent"}`;
  try {
    if (sessionStorage.getItem(sessionKey) === "1") return;
  } catch {
    // Private mode can block storage; report anyway — the upsert is idempotent.
  }

  try {
    const response = await fetch(path, {
      method: "POST",
      credentials: "include",
    });
    if (!response.ok) {
      logger.debug("pwa_usage_report_rejected", {
        portal,
        status: response.status,
      });
      return;
    }
    try {
      sessionStorage.setItem(sessionKey, "1");
    } catch {
      // Storage denied — the next report in this session is still harmless.
    }
  } catch (err: unknown) {
    logger.debug("pwa_usage_report_failed", {
      portal,
      error: err instanceof Error ? err.message : String(err),
    });
  }
}
