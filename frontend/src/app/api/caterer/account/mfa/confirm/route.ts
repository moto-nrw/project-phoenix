import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { caterer } from "~/lib/caterer/route-wrapper.server";
import { installLoginSession } from "~/lib/login-route.server";
import { createLogger } from "~/lib/logger";
import { captureBffException } from "~/lib/sentry-bff.server";
import { catererPortal } from "~/server/auth/caterer";
import { sessionMutationOriginError } from "~/server/auth/same-origin";

const logger = createLogger({ component: "CatererMFAConfirmRoute" });

// Switching the caterer's two-factor sign-in on (#4497). The backend answers
// with a new session with the second factor proven; the BFF installs it, so
// no token reaches the browser (#5005). When the code was right but the new
// session cannot be installed, the answer says so: the second factor is on
// and the caterer must sign in again.
export const POST = catererPortal.withAuth(async (request) => {
  const originError = sessionMutationOriginError(request);
  if (originError) return originError;
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return problemResponse(400, "Invalid JSON", request);
  }
  try {
    const backend = await caterer.backendFetch(
      request,
      "/caterer/account/mfa/confirm",
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
    const failure = await installLoginSession(request, data, catererPortal);
    return Response.json({ session_renewed: failure === null });
  } catch (error) {
    captureBffException(error, request);
    logger.error("caterer_mfa_confirm_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "MFA confirm unavailable", request);
  }
});
