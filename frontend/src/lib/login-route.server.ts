import "server-only";
import type { NextRequest } from "next/server";
import type { ResponseRouteHandler } from "~/server/auth/route-handler";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "LoginRoute" });

interface LoginOptions {
  withResponse: (handler: ResponseRouteHandler) => ResponseRouteHandler;
  setSession: (token: string, refreshToken: string) => Promise<void>;
  rollbackPath?: string;
  envelope?: boolean;
  challenges?: boolean;
  organizations?: boolean;
}

/** A failed handoff never exposes its freshly minted credentials. */
export async function revokeMintedSession(
  request: NextRequest,
  refreshToken: string,
  path = "/auth/logout",
) {
  try {
    const response = await fetch(`${getServerApiUrl()}${path}`, {
      method: "POST",
      headers: {
        ...getClientForwardHeaders(request),
        Authorization: `Bearer ${refreshToken}`,
      },
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok)
      logger.error("login_session_rollback_failed", {
        status: response.status,
      });
  } catch {
    logger.error("login_session_rollback_failed", {});
  }
}

export async function installLoginSession(
  request: NextRequest,
  data: Record<string, unknown>,
  setSession: LoginOptions["setSession"],
  rollbackPath = "/auth/logout",
): Promise<Response | null> {
  if (
    typeof data.access_token !== "string" ||
    !data.access_token ||
    typeof data.refresh_token !== "string" ||
    !data.refresh_token
  )
    throw new Error("Missing session credentials");
  try {
    await setSession(data.access_token, data.refresh_token);
  } catch {
    await revokeMintedSession(request, data.refresh_token, rollbackPath);
    return problemResponse(500, "Session creation failed", request);
  }
  return null;
}

export function createLoginRoute(path: string, options: LoginOptions) {
  return options.withResponse(async (request: NextRequest) => {
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
        ...(path.endsWith("/mfa/enroll/confirm") &&
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
        const payload = (await backend.json().catch(() => {
          throw new Error("Invalid login response");
        })) as Record<string, unknown>;
        const data = options.envelope
          ? (payload.data as Record<string, unknown>)
          : payload;
        const json = (body: Record<string, unknown>) =>
          Response.json(
            options.envelope ? { status: "success", data: body } : body,
          );
        if (
          options.organizations &&
          options.challenges &&
          data.status === "organization_selection_required"
        ) {
          if (!Array.isArray(data.organizations))
            throw new Error("Invalid organization selection");
          response = json({
            status: data.status,
            organizations: data.organizations.map(
              (value: { id: unknown; name: unknown }) => ({
                id: value.id,
                name: value.name,
              }),
            ),
          });
        } else if (options.challenges && data.status === "mfa_required") {
          if (
            typeof data.challenge_token !== "string" ||
            !data.challenge_token ||
            data.access_token ||
            data.refresh_token
          )
            throw new Error("Invalid MFA challenge response");
          response = json({
            status: data.status,
            challenge_token: data.challenge_token,
            masked_email: data.masked_email,
            ...(options.organizations
              ? { organization_id: data.organization_id }
              : {}),
            trusted_device_enabled: data.trusted_device_enabled,
            trusted_device_days: data.trusted_device_days,
          });
        } else if (
          options.challenges &&
          data.status === "mfa_enrollment_required"
        ) {
          if (
            typeof data.access_token !== "string" ||
            !data.access_token ||
            data.refresh_token
          )
            throw new Error("Invalid enrollment response");
          // A limited enrollment proof is not a completed session token.
          response = json({
            status: data.status,
            access_token: data.access_token,
            masked_email: data.masked_email,
            ...(options.organizations
              ? { organization_id: data.organization_id }
              : {}),
            mfa_enrollment_required: true,
          });
        } else {
          const failure = await installLoginSession(
            request,
            data,
            options.setSession,
            options.rollbackPath,
          );
          if (failure) return failure;
          response = json({ status: "authenticated" });
        }
      }
      // Includes trusted-device creation/expiry, independent of the Auth.js cookie.
      for (const cookie of backend.headers.getSetCookie())
        response.headers.append("Set-Cookie", cookie);
      return response;
    } catch (error) {
      captureBffException(error, request);
      logger.error("login_failed", {
        path,
        error: error instanceof Error ? error.message : String(error),
      });
      return problemResponse(500, "Login unavailable", request);
    }
  });
}
