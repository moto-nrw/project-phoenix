import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request, params) => {
    const tenantSlug = params.tenantSlug;
    if (typeof tenantSlug !== "string" || !tenantSlug) {
      return problemResponse(400, "tenant slug is required", request);
    }
    const phaseId = request.nextUrl.searchParams.get("phaseId");
    const lateInvite = request.nextUrl.searchParams.get("late_invite")?.trim();
    const path = phaseId
      ? `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}/${encodeURIComponent(phaseId)}`
      : `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}`;
    return lateInvite
      ? `${path}?late_invite=${encodeURIComponent(lateInvite)}`
      : path;
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
