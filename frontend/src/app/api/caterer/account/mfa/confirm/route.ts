import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { installLoginSession } from "~/lib/login-route.server";
import { createLogger } from "~/lib/logger";
import { captureBffException } from "~/lib/sentry-bff.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { setCatererSession, withCatererAuth } from "~/server/auth/caterer";
import { sessionMutationOriginError } from "~/server/auth/same-origin";

const logger = createLogger({ component: "CatererMFAConfirmRoute" });

// Switching the caterer's two-factor sign-in on (#4497). The backend answers
// with a new session with the second factor proven; the BFF installs it, so
// no token reaches the browser (#5005). When the code was right but the new
// session cannot be installed, the answer says so: the second factor is on
// and the caterer must sign in again.
export const POST = withCatererAuth(async (request) => {
  const originError = sessionMutationOriginError(request);
  if (originError) return originError;
  const token = request.auth?.user?.token;
  if (!token) return problemResponse(401, "Unauthorized", request);
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return problemResponse(400, "Invalid JSON", request);
  }
  try {
    const backend = await fetch(
      `${getServerApiUrl()}/caterer/account/mfa/confirm`,
      {
        method: "POST",
        headers: {
          ...getClientForwardHeaders(request),
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      },
    );
    if (!backend.ok) return forwardBackendResponse(backend, request);
    // Parse errors must not put bearer material in an exception message.
    const data = (await backend.json().catch(() => {
      throw new Error("Invalid MFA confirm response");
    })) as Record<string, unknown>;
    const failure = await installLoginSession(request, data, setCatererSession);
    return Response.json({ session_renewed: failure === null });
  } catch (error) {
    captureBffException(error, request);
    logger.error("caterer_mfa_confirm_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "MFA confirm unavailable", request);
  }
});
