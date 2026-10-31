import "server-only";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingClientForwardHeaders } from "~/lib/client-headers.server";
import type { JwtPayload } from "./jwt-payload";

type Portal = "tenant" | "platform" | "parent" | "school" | "caterer" | "org";

/** Only backend-verified claims may cross a client token-handoff boundary. */
export async function validateSessionToken(
  accessToken: string,
  portal: Portal,
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
    const response = await fetch(`${getServerApiUrl()}/auth/session/validate`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${accessToken}`,
        ...(await incomingClientForwardHeaders()),
      },
      body: JSON.stringify({
        access_token: accessToken,
        refresh_token: refreshToken,
        portal,
      }),
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
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
    const scope = claims.scope ?? "";
    if (
      portal === "tenant"
        ? !["", "tenant", "org"].includes(scope)
        : scope !== portal
    )
      return null;
    return claims;
  } catch {
    return null;
  }
}

/** live: the family backs the token; ended: revoked or expired; unknown: no answer. */
export type SessionFamilyState = "live" | "ended" | "unknown";

/** The validation portal of a session's scope claim. */
export function portalOfScope(scope: unknown): Portal {
  switch (scope) {
    case "platform":
    case "parent":
    case "school":
    case "caterer":
    case "org":
      return scope;
    default:
      return "tenant";
  }
}

/**
 * Asks Identity & Access whether the refresh-token family an access token
 * names still backs it (#5399). Without the refresh token the backend reads
 * the family itself; with it, a rotated token of a live family would fail.
 * The BFF writes or shows a session only for a live family, so the answer of
 * a request that outlived its logout sets no cookie and reads as ended.
 */
export async function sessionFamilyState(
  accessToken: string,
  portal: Portal,
): Promise<SessionFamilyState> {
  if (!accessToken || accessToken.length > 16000) return "ended";
  try {
    const response = await fetch(`${getServerApiUrl()}/auth/session/validate`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${accessToken}`,
        ...(await incomingClientForwardHeaders()),
      },
      body: JSON.stringify({ access_token: accessToken, portal }),
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (response.ok) return "live";
    return response.status === 401 ? "ended" : "unknown";
  } catch {
    return "unknown";
  }
}
