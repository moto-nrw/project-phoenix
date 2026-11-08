import "server-only";
import { sendBackend } from "~/lib/backend-client.server";
import { decodeJwtPayload, type JwtPayload } from "./jwt-payload";
import { acceptsScope, type ValidationPortal } from "./portals";

/** Only backend-verified claims may cross a client token-handoff boundary. */
export async function validateSessionToken(
  accessToken: string,
  portal: ValidationPortal,
  refreshToken?: string,
): Promise<JwtPayload | null> {
  if (
    typeof accessToken !== "string" ||
    !accessToken ||
    accessToken.length > 16000 ||
    (refreshToken !== undefined &&
      (typeof refreshToken !== "string" ||
        !refreshToken ||
        refreshToken.length > 16000))
  )
    return null;
  try {
    const response = await sendBackend("/auth/session/validate", accessToken, {
      method: "POST",
      json: { access_token: accessToken, refresh_token: refreshToken, portal },
      timeout: "session",
    });
    if (!response.ok) return null;
    const claims = (await response.json()) as JwtPayload;
    if (
      !claims ||
      !claims.id ||
      typeof claims.exp !== "number" ||
      claims.exp * 1000 <= Date.now()
    )
      return null;
    if (!acceptsScope(portal, claims.scope)) return null;
    return claims;
  } catch {
    return null;
  }
}

/** live: the family backs the token; ended: revoked; unknown: no answer, or an expired token. */
export type SessionFamilyState = "live" | "ended" | "unknown";

/**
 * Asks Identity & Access whether the refresh-token family an access token
 * names still backs it (#5399). Without the refresh token the backend reads
 * the family itself; with it, a rotated token of a live family would fail.
 * The BFF writes or shows a session only for a live family, so the answer of
 * a request that outlived its logout sets no cookie and reads as ended.
 */
export async function sessionFamilyState(
  accessToken: string,
  portal: ValidationPortal,
): Promise<SessionFamilyState> {
  if (!accessToken || accessToken.length > 16000) return "ended";
  // Identity & Access rejects an expired token before it reads the family,
  // so its refusal says nothing about the family. The session's refresh
  // decides; a revoked family ends there.
  const expiry = decodeJwtPayload(accessToken)?.exp;
  if (typeof expiry === "number" && expiry * 1000 <= Date.now()) {
    return "unknown";
  }
  try {
    const response = await sendBackend("/auth/session/validate", accessToken, {
      method: "POST",
      json: { access_token: accessToken, portal },
      timeout: "session",
    });
    if (response.ok) return "live";
    return response.status === 401 ? "ended" : "unknown";
  } catch {
    return "unknown";
  }
}
