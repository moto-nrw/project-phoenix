import "server-only";
import type { JWT } from "next-auth/jwt";
import { getServerApiUrl } from "~/lib/server-api-url";
import {
  authLogger,
  parseJwtPayload,
  syncTokenFromPayload,
} from "./jwt-claims";
import type { BackendEndpoint } from "./portals";
import { accessTokenExpiresAt } from "./session-lifetime";
import {
  applyRotation,
  getRefreshAuditHeaders,
  isRenewalForced,
  rotateRefreshToken,
} from "./token-refresh";
import { validateSessionToken } from "./token-validation";

// Admin staff-view preview (#2893), tenant portal only.
//
// While a preview is active, `token.token` holds the READ-ONLY preview JWT
// (the target staff member's identity + permissions, minted by the backend),
// the admin's own access token is parked in `previewAdminToken`, and
// `refreshToken` stays the admin's. The preview lives only as long as the
// admin session: near expiry the callback refreshes the ADMIN token and asks
// the backend for a fresh preview token — no refresh token ever exists in
// the target's name.

const PREVIEW_REFRESH_BUFFER_MS = 5 * 60 * 1000;
const PREVIEW_ADMIN_TOKEN_MIN_TTL_MS = 60 * 1000;
const PREVIEW_REQUEST_TIMEOUT_MS = 5_000;

export interface PreviewStartUpdate {
  accessToken: string;
  expiresIn: number;
  // Account IDs are int64 on the backend and stay strings on this side —
  // a JavaScript number would round IDs beyond 2^53.
  targetAccountId: string;
  targetName: string;
}

export function isPreviewStartUpdate(
  value: unknown,
): value is PreviewStartUpdate {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.accessToken === "string" &&
    typeof v.expiresIn === "number" &&
    typeof v.targetAccountId === "string" &&
    v.targetAccountId.length > 0 &&
    typeof v.targetName === "string"
  );
}

export function isPreviewActive(token: JWT): boolean {
  return typeof token.previewTargetAccountId === "string";
}

function syncPreviewIdentity(token: JWT, accessToken: string) {
  const payload = parseJwtPayload(accessToken);
  if (!payload) return;
  token.id = String(payload.id);
  if (payload.sub) token.email = payload.sub;
  syncTokenFromPayload(token, payload, "account");
}

/** Swap the session onto the preview token, parking the admin's own state. */
export async function applyPreviewStart(
  token: JWT,
  update: PreviewStartUpdate,
): Promise<void> {
  const payload = await validateSessionToken(update.accessToken, "tenant");
  const admin =
    typeof token.token === "string"
      ? await validateSessionToken(token.token, "tenant")
      : null;
  // Only genuine read-only preview tokens may be injected here — anything
  // else would smuggle a full session under the preview flag. The session
  // stays as it is. The BFF checks the server update result and closes the
  // just-recorded preview if the session did not enter the requested target.
  if (
    !payload?.read_only ||
    !admin ||
    admin.read_only ||
    String(payload.acting_admin_id) !== String(admin.id) ||
    payload.tenant_id !== admin.tenant_id ||
    String(payload.id) !== update.targetAccountId
  ) {
    authLogger.warn("staff_preview_start_rejected", {
      reason: "token is not a read-only preview token",
    });
    return;
  }
  token.previewAdminToken = token.token;
  token.previewAdminTokenExpiry = token.tokenExpiry;
  token.previewAdminId = token.id;
  token.previewAdminEmail = token.email ?? undefined;
  token.previewTargetAccountId = update.targetAccountId;
  token.previewTargetName = update.targetName;
  token.token = update.accessToken;
  token.tokenExpiry = payload.exp! * 1000;
  token.id = String(payload.id);
  if (payload.sub) token.email = payload.sub;
  syncTokenFromPayload(token, payload, "account");
  authLogger.info("staff_preview_started", {
    target_account_id: update.targetAccountId,
  });
}

/** Restore the parked admin state and clear every preview field. */
export function restoreAdminFromPreview(token: JWT): void {
  token.token = token.previewAdminToken ?? "";
  token.tokenExpiry = token.previewAdminTokenExpiry;
  token.id = token.previewAdminId;
  token.email = token.previewAdminEmail;
  const payload = token.token ? parseJwtPayload(token.token) : null;
  if (payload) syncTokenFromPayload(token, payload, "account");
  token.previewTargetAccountId = undefined;
  token.previewTargetName = undefined;
  token.previewAdminToken = undefined;
  token.previewAdminTokenExpiry = undefined;
  token.previewAdminId = undefined;
  token.previewAdminEmail = undefined;
  authLogger.info("staff_preview_ended");
}

/**
 * Record the end of a preview that the SESSION ended on its own — the target
 * lost eligibility, or the admin session itself died. The interactive
 * "Vorschau beenden" button posts the same call from the browser; this is the
 * same ending, just without a click, and it must reach the audit trail the
 * same way. The endpoint is public and token-proved: the signed preview token
 * is the credential, so the end is recordable even when every admin token has
 * expired (laptop closed for a week mid-preview). Best effort: the preview is
 * over regardless of what this call answers, and the backend records one end
 * per preview instance however often it is asked (unique index on the
 * preview id).
 */
async function recordPreviewEnd(
  previewToken: string | undefined,
): Promise<void> {
  if (!previewToken) return;
  try {
    // Without the client's audit context the automatic end lands with the
    // Docker-internal IP and Node as browser.
    const response = await fetch(
      `${getServerApiUrl()}/auth/staff-preview/end`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(await getRefreshAuditHeaders()),
        },
        body: JSON.stringify({ preview_token: previewToken }),
        signal: AbortSignal.timeout(PREVIEW_REQUEST_TIMEOUT_MS),
      },
    );
    if (!response.ok) {
      authLogger.warn("staff_preview_auto_end_not_recorded", {
        status: response.status,
      });
    }
  } catch (err) {
    authLogger.warn("staff_preview_auto_end_error", {
      error: err instanceof Error ? err.message : String(err),
    });
  }
}

/**
 * End a preview because the SESSION is lost, not because anyone clicked:
 * the admin refresh failed terminally or the refresh token expired. The
 * session callback strips the token right after, so this is the last moment
 * the preview token is in hand — without recording here, the audit trail
 * keeps a "started" that never ends. Recorded first (the signed preview
 * token is the credential), then the admin state is restored so no preview
 * field survives into the dead session.
 */
export async function endPreviewOnSessionLoss(token: JWT): Promise<void> {
  await recordPreviewEnd(token.token);
  restoreAdminFromPreview(token);
}

/**
 * Keep an active preview session alive: refresh the admin token when needed
 * and re-mint the preview token near its expiry. Returns true while the
 * preview stays active (caller returns the token) and false when the preview
 * ended (caller falls through to the regular refresh path with the restored
 * admin state).
 */
export async function maintainPreviewSession(
  token: JWT,
  refresh: BackendEndpoint,
  now: number,
): Promise<boolean> {
  const previewExpiry = token.tokenExpiry ?? 0;
  // A forced renewal re-proves the admin session the preview rests on.
  const forced = isRenewalForced();
  if (!forced && now < previewExpiry - PREVIEW_REFRESH_BUFFER_MS) return true;

  let adminAccessToken = token.previewAdminToken;
  const adminExpiry = token.previewAdminTokenExpiry ?? 0;
  if (
    forced ||
    !adminAccessToken ||
    now > adminExpiry - PREVIEW_ADMIN_TOKEN_MIN_TTL_MS
  ) {
    const rotation = await rotateRefreshToken(token, refresh);
    if (rotation.status === "success") {
      adminAccessToken = applyRotation(token, rotation);
      token.previewAdminToken = adminAccessToken;
      token.previewAdminTokenExpiry = accessTokenExpiresAt(adminAccessToken);
    } else if (rotation.status === "terminal") {
      // The admin session itself is gone — everything ends, preview included.
      await endPreviewOnSessionLoss(token);
      token.error = "RefreshTokenError";
      token.needsRefresh = true;
      return true;
    } else {
      return true; // transient — retry on the next callback
    }
  }

  // Re-mint the preview token as the admin. The mint rotates nothing, so a
  // concurrent double-mint is harmless.
  try {
    const response = await fetch(`${getServerApiUrl()}/auth/staff-preview`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${adminAccessToken}`,
        "Content-Type": "application/json",
        // A re-mint normally continues the same preview and writes no start
        // event — but if the backend ever treats it as a new one, the audit
        // row must carry the real client, not the Docker-internal hop.
        ...(await getRefreshAuditHeaders()),
      },
      // The expiring token comes along as previous_token: it proves this is
      // the SAME preview instance, so the backend renews it instead of
      // recording a second "preview started" in the audit trail.
      body: JSON.stringify({
        account_id: token.previewTargetAccountId,
        previous_token: token.token,
      }),
      signal: AbortSignal.timeout(PREVIEW_REQUEST_TIMEOUT_MS),
    });

    if (response.ok) {
      const data = (await response.json()) as {
        access_token: string;
        expires_in: number;
        target_name: string;
      };
      token.token = data.access_token;
      token.tokenExpiry = Date.now() + data.expires_in * 1000;
      token.previewTargetName = data.target_name;
      syncPreviewIdentity(token, data.access_token);
      authLogger.info("staff_preview_token_reminted");
      return true;
    }

    if (response.status === 401) {
      // Stale admin access token — drop it so the next callback refreshes.
      token.previewAdminToken = undefined;
      token.previewAdminTokenExpiry = undefined;
      return true;
    }

    // 403/404/…: the target is no longer previewable (deactivated, role
    // changed, admin rights revoked). This ending is as real as one the admin
    // clicks, so it gets the same audit entry, recorded while the preview
    // token is still in hand — it proves which preview is being closed.
    authLogger.warn("staff_preview_remint_rejected", {
      status: response.status,
    });
    await recordPreviewEnd(token.token);
    restoreAdminFromPreview(token);
    return false;
  } catch (err) {
    authLogger.warn("staff_preview_remint_error", {
      error: err instanceof Error ? err.message : String(err),
    });
    return true; // transient — retry on the next callback
  }
}
