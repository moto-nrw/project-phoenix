import "server-only";
import type { NextRequest } from "next/server";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { sendBackend } from "~/lib/backend-client.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import {
  BackendUnavailableError,
  handleApiError,
  problemResponse,
} from "~/lib/api-response-error.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import { withoutSessionCookies } from "~/server/auth/session-cookie-writes";
import { createLogger } from "~/lib/logger";
import type { PortalAuth } from "~/server/auth/portal-auth";
import type { BackendAuthStep } from "~/server/auth/portals";
import {
  installMintedSession,
  logoutRollback,
  mintedPair,
} from "./session-transition.server";

const logger = createLogger({ component: "LoginRoute" });

/** What a login needs of its portal; every portal entry module exports it. */
export type LoginPortal = Pick<
  PortalAuth,
  "portal" | "setSession" | "withSessionResponse"
>;

/** A pre-session step of a portal's login the BFF forwards. */
export type LoginStep = Exclude<BackendAuthStep, "refresh">;

/** The backend's remember-device cookie, read by its login and MFA steps. */
const TRUSTED_DEVICE_COOKIE = "mfa_trust_device";

/**
 * Installs the token pair of a completed login as the portal's session; a
 * failed install revokes the pair with that portal's logout.
 */
export function installLoginSession(
  request: NextRequest,
  data: Record<string, unknown>,
  { portal, setSession }: Pick<LoginPortal, "portal" | "setSession">,
): Promise<Response | null> {
  const pair = mintedPair(data);
  return installMintedSession(
    request,
    () => setSession(pair.access_token, pair.refresh_token),
    logoutRollback(portal.backend.logout, pair),
  );
}

/**
 * The BFF route of one login step of a portal. Only the password step can
 * answer with a next step (organization choice, MFA challenge, enrollment);
 * only the enrollment confirmation carries the limited enrollment proof.
 */
export function createPortalLoginRoute(login: LoginPortal, step: LoginStep) {
  const { backend } = login.portal;
  const path = `${backend.authPrefix}/${step}`;
  const envelope = (backend.enveloped as readonly BackendAuthStep[]).includes(
    step,
  );
  const challenges = step === "login";
  const organizations = challenges && backend.organizationSelection === true;
  return login.withSessionResponse(async (request: NextRequest) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    let body: unknown;
    try {
      body = await request.json();
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }
    try {
      const trustedDevice = request.cookies.get(TRUSTED_DEVICE_COOKIE)?.value;
      const enrollmentProof =
        step === "mfa/enroll/confirm"
          ? request.headers.get("authorization")
          : null;
      const backend = await sendBackend(path, null, {
        method: "POST",
        json: body,
        request,
        timeout: "login",
        headers: {
          ...(trustedDevice && {
            Cookie: `${TRUSTED_DEVICE_COOKIE}=${trustedDevice}`,
          }),
          ...(enrollmentProof && { Authorization: enrollmentProof }),
        },
      });
      let response: Response;
      if (!backend.ok) {
        response = forwardBackendResponse(backend, request);
      } else {
        // Parse errors must not put bearer material in an exception message.
        const payload = (await backend.json().catch(() => {
          throw new Error("Invalid login response");
        })) as Record<string, unknown>;
        const data = envelope
          ? (payload.data as Record<string, unknown>)
          : payload;
        const json = (body: Record<string, unknown>) =>
          Response.json(envelope ? { status: "success", data: body } : body);
        if (
          organizations &&
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
        } else if (challenges && data.status === "mfa_required") {
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
            ...(organizations ? { organization_id: data.organization_id } : {}),
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
          response = json({
            status: data.status,
            access_token: data.access_token,
            masked_email: data.masked_email,
            ...(organizations ? { organization_id: data.organization_id } : {}),
            mfa_enrollment_required: true,
          });
        } else {
          const failure = await installLoginSession(request, data, login);
          if (failure) return failure;
          response = json({ status: "authenticated" });
        }
      }
      // Includes trusted-device creation/expiry, independent of the Auth.js
      // cookie; a session cookie leaves only via its gate (#5399).
      for (const cookie of withoutSessionCookies(
        backend.headers.getSetCookie(),
      ))
        response.headers.append("Set-Cookie", cookie);
      return response;
    } catch (error) {
      logger.error("login_failed", {
        path,
        error: error instanceof Error ? error.message : String(error),
      });
      if (error instanceof BackendUnavailableError) {
        return handleApiError(error, request);
      }
      captureBffException(error, request);
      return problemResponse(500, "Login unavailable", request);
    }
  });
}
