import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { installLoginSession } from "~/lib/login-route.server";
import { createLogger } from "~/lib/logger";
import { captureBffException } from "~/lib/sentry-bff.server";
import { traeger } from "~/lib/traeger/route-wrapper.server";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { traegerPortal } from "~/server/auth/traeger";

const logger = createLogger({ component: "TraegerStepUpVerifyRoute" });

// Code step in the running Träger session (#7068). The backend answers with
// the rotated session whose factor the code proved; the BFF installs it in
// place of the old one, so no token reaches the browser (#5005). When the
// code was right but the new session cannot be installed, the answer says
// so: the old session is retired and the person signs in again.
export const POST = traegerPortal.withAuth(async (request) => {
  const originError = sessionMutationOriginError(request);
  if (originError) return originError;
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return problemResponse(400, "Invalid JSON", request);
  }
  try {
    const backend = await traeger.backendFetch(
      request,
      "/traeger/auth/mfa/step-up/verify",
      {
        method: "POST",
        json: body,
        retry: { off: "a renewed session would redeem the code twice" },
      },
    );
    if (!backend.ok) return forwardBackendResponse(backend, request);
    // Parse errors must not put bearer material in an exception message.
    const data = (await backend.json().catch(() => {
      throw new Error("Invalid step-up response");
    })) as Record<string, unknown>;
    const failure = await installLoginSession(request, data, traegerPortal);
    return Response.json({ data: { session_renewed: failure === null } });
  } catch (error) {
    captureBffException(error, request);
    logger.error("traeger_step_up_verify_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Step-up unavailable", request);
  }
});
