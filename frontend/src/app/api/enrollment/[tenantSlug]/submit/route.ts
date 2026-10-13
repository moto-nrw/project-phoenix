import { analyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const POST = createPublicJsonProxy({
  method: "POST",
  path: (_request, params) => {
    const tenantSlug = params.tenantSlug;
    if (typeof tenantSlug !== "string" || !tenantSlug) {
      return problemResponse(400, "tenant slug is required", _request);
    }
    return `/api/enrollment/${encodeURIComponent(tenantSlug)}/submit`;
  },
  additionalHeaders: (request) => {
    const forwardedFor = canonicalForwardedFor(request.headers);
    return {
      ...(forwardedFor ? { "X-Forwarded-For": forwardedFor } : {}),
      ...analyticsSessionHeaders(request.headers),
    };
  },
});
