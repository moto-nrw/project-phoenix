import { getServerApiUrl } from "~/lib/server-api-url";
import type { JwtPayload } from "./shared";

type Portal = "tenant" | "platform" | "parent" | "school";

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
