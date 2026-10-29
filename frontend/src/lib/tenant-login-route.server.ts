import "server-only";
import type { NextRequest } from "next/server";
import { setTenantSession, withTenantSessionResponse } from "~/server/auth";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantLoginRoute" });

/** Public login endpoints mint sessions, but must never authenticate/renew an
 * old cookie before replacing it. Only challenge/enrollment proofs cross JSON.
 */
export function createTenantLoginRoute(
  path:
    | "/auth/login"
    | "/auth/mfa/verify"
    | "/auth/mfa/enroll/confirm"
    | "/auth/passkeys/login/verify",
) {
  const challenges = path === "/auth/login";
  return withTenantSessionResponse(async (request: NextRequest) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    let body: unknown;
    try {
      body = await request.json();
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }
    try {
      const headers = {
        ...getClientForwardHeaders(request),
        "Content-Type": "application/json",
        ...(request.headers.get("cookie")
          ? { Cookie: request.headers.get("cookie")! }
          : {}),
        ...(path === "/auth/mfa/enroll/confirm" &&
        request.headers.get("authorization")
          ? { Authorization: request.headers.get("authorization")! }
          : {}),
      };
      const backend = await fetch(`${getServerApiUrl()}${path}`, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      });
      let response: Response;
      if (!backend.ok) {
        response = forwardBackendResponse(backend, request);
      } else {
        // Parse errors must not put bearer material in an exception message.
        const data = (await backend.json().catch(() => {
          throw new Error("Invalid login response");
        })) as Record<string, unknown>;
        if (challenges && data.status === "mfa_required") {
          if (
            typeof data.challenge_token !== "string" ||
            !data.challenge_token ||
            data.access_token ||
            data.refresh_token
          )
            throw new Error("Invalid MFA challenge response");
          response = Response.json({
            status: data.status,
            challenge_token: data.challenge_token,
            masked_email: data.masked_email,
            trusted_device_enabled: data.trusted_device_enabled,
            trusted_device_days: data.trusted_device_days,
          });
        } else if (challenges && data.status === "mfa_enrollment_required") {
          if (
            typeof data.access_token !== "string" ||
            !data.access_token ||
            data.refresh_token
          )
            throw new Error("Invalid enrollment response");
          // A limited enrollment proof is not a completed session token.
          response = Response.json({
            status: data.status,
            access_token: data.access_token,
            masked_email: data.masked_email,
            mfa_enrollment_required: true,
          });
        } else {
          if (
            typeof data.access_token !== "string" ||
            typeof data.refresh_token !== "string" ||
            !data.access_token ||
            !data.refresh_token
          )
            throw new Error("Missing session credentials");
          try {
            await setTenantSession(data.access_token, data.refresh_token);
          } catch {
            // A newly minted refresh family must not survive a failed handoff.
            try {
              const rollback = await fetch(`${getServerApiUrl()}/auth/logout`, {
                method: "POST",
                headers: {
                  ...getClientForwardHeaders(request),
                  Authorization: `Bearer ${data.refresh_token}`,
                },
                signal: AbortSignal.timeout(5000),
              });
              if (!rollback.ok)
                logger.error("login_session_rollback_failed", {
                  status: rollback.status,
                });
            } catch {
              logger.error("login_session_rollback_failed", {});
            }
            return problemResponse(500, "Session creation failed", request);
          }
          response = Response.json({ status: "authenticated" });
        }
      }
      // Includes trusted-device creation/expiry, independent of the Auth.js cookie.
      for (const cookie of backend.headers.getSetCookie())
        response.headers.append("Set-Cookie", cookie);
      return response;
    } catch (error) {
      captureBffException(error, request);
      logger.error("tenant_login_failed", {
        path,
        error: error instanceof Error ? error.message : String(error),
      });
      return problemResponse(500, "Login unavailable", request);
    }
  });
}
