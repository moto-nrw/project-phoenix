import { analyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { canonicalForwardedFor } from "~/lib/client-headers.server";

// Public holiday registration (#4661): the one link of a holiday period, no
// session. The backend counts attempts per client IP, so the canonical
// address travels along.
const path = (
  request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const token = params.token;
  if (typeof token !== "string" || !token) {
    return problemResponse(400, "token required", request);
  }
  return `/traeger/holiday-registrations/${encodeURIComponent(token)}`;
};

export const GET = createPublicJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  contentTypeOnGet: false,
});

export const POST = createPublicJsonProxy({
  method: "POST",
  path,
  additionalHeaders: (request) => {
    const forwardedFor = canonicalForwardedFor(request.headers);
    return {
      ...(forwardedFor ? { "X-Forwarded-For": forwardedFor } : {}),
      ...analyticsSessionHeaders(request.headers),
    };
  },
});
