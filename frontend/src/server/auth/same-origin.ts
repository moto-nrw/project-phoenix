import "server-only";
import type { NextRequest } from "next/server";
import { frontendOrigin } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-response-error.server";

/** Server-side Auth.js actions skip its CSRF check. Check the browser origin. */
export function sessionMutationOriginError(
  request: NextRequest,
): Response | null {
  const origin = request.headers.get("origin");
  if (
    origin !== frontendOrigin(request) ||
    request.headers.get("sec-fetch-site") === "cross-site"
  ) {
    return problemResponse(403, "Same-origin request required", request);
  }
  return null;
}
