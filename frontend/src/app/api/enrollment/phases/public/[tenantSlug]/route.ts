import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (_request, params) => {
    const tenantSlug = params.tenantSlug;
    if (typeof tenantSlug !== "string" || !tenantSlug) {
      return problemResponse(400, "tenant slug is required", _request);
    }
    return `/api/enrollment/phases/public/${encodeURIComponent(tenantSlug)}`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
