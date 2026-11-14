import "server-only";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { installLoginSession } from "~/lib/login-route.server";
import { createLogger } from "~/lib/logger";
import type { makePortalHandlers } from "~/lib/portal-handlers.server";
import { captureBffException } from "~/lib/sentry-bff.server";
import type { PortalAuth } from "~/server/auth/portal-auth";
import { sessionMutationOriginError } from "~/server/auth/same-origin";

interface FactorConfirmRoute {
  portal: Pick<PortalAuth, "portal" | "setSession" | "withAuth">;
  handlers: Pick<ReturnType<typeof makePortalHandlers>, "backendFetch">;
  /** The backend route that redeems the code and answers the new session. */
  path: string;
  /** Why a backend 401 is not repeated with a renewed session. */
  retry: { off: string };
  log: { component: string; event: string };
}

/**
 * The BFF route that switches a voluntary second factor on (caterer #4497,
 * parents #7051). The backend answers with a new session with the second
 * factor proven; the BFF installs it, so no token reaches the browser
 * (#5005). When the code was right but the new session cannot be installed,
 * the answer says so: the second factor is on and the person must sign in
 * again.
 */
export function createFactorConfirmRoute({
  portal,
  handlers,
  path,
  retry,
  log,
}: FactorConfirmRoute) {
  const logger = createLogger({ component: log.component });
  return portal.withAuth(async (request) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    let body: unknown;
    try {
      body = await request.json();
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }
    try {
      const backend = await handlers.backendFetch(request, path, {
        method: "POST",
        json: body,
        retry,
      });
      if (!backend.ok) return forwardBackendResponse(backend, request);
      // Parse errors must not put bearer material in an exception message.
      const data = (await backend.json().catch(() => {
        throw new Error("Invalid MFA confirm response");
      })) as Record<string, unknown>;
      const failure = await installLoginSession(request, data, portal);
      return Response.json({ session_renewed: failure === null });
    } catch (error) {
      captureBffException(error, request);
      logger.error(log.event, {
        error: error instanceof Error ? error.message : String(error),
      });
      return problemResponse(500, "MFA confirm unavailable", request);
    }
  });
}

/**
 * The body of the route that switches the factor off. The current password
 * is a credential check: a wrong one comes back as 401 with its own code and
 * stays on the form, like the password change.
 */
export function currentPasswordBody(body: unknown) {
  return {
    current_password:
      (body as { currentPassword?: string }).currentPassword ?? "",
  };
}
