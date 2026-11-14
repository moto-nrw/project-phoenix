import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { parent } from "~/lib/parent/route-wrapper.server";
import { installLoginSession } from "~/lib/login-route.server";
import { createLogger } from "~/lib/logger";
import { captureBffException } from "~/lib/sentry-bff.server";
import { parentPortal } from "~/server/auth/parent";
import { sessionMutationOriginError } from "~/server/auth/same-origin";

const logger = createLogger({ component: "ParentMFAConfirmRoute" });

// Switching the parents' sign-in with a code on (#7051). The backend answers
// with a new session with the second factor proven; the BFF installs it, so
// no token reaches the browser (#5005). When the code was right but the new
// session cannot be installed, the answer says so: the second factor is on
// and the parent must sign in again.
export const POST = parentPortal.withAuth(async (request) => {
  const originError = sessionMutationOriginError(request);
  if (originError) return originError;
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return problemResponse(400, "Invalid JSON", request);
  }
  try {
    const backend = await parent.backendFetch(
      request,
      "/parent/me/mfa/confirm",
      {
        method: "POST",
        json: body,
        retry: { off: "a renewed session would send the code twice" },
      },
    );
    if (!backend.ok) return forwardBackendResponse(backend, request);
    // Parse errors must not put bearer material in an exception message.
    const data = (await backend.json().catch(() => {
      throw new Error("Invalid MFA confirm response");
    })) as Record<string, unknown>;
    const failure = await installLoginSession(request, data, parentPortal);
    return Response.json({ session_renewed: failure === null });
  } catch (error) {
    captureBffException(error, request);
    logger.error("parent_mfa_confirm_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "MFA confirm unavailable", request);
  }
});
