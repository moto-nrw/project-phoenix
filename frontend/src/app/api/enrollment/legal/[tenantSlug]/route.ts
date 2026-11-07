import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";
import {
  LATE_INVITE_QUERY_PARAM,
  withLateInviteQuery,
} from "~/lib/enrollment-http";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request, params) => {
    const tenantSlug = params.tenantSlug;
    if (typeof tenantSlug !== "string" || !tenantSlug) {
      return problemResponse(400, "tenant slug is required", request);
    }
    const phaseId = request.nextUrl.searchParams.get("phaseId");
    const path = phaseId
      ? `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}/${encodeURIComponent(phaseId)}`
      : `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}`;
    return withLateInviteQuery(
      path,
      request.nextUrl.searchParams.get(LATE_INVITE_QUERY_PARAM),
    );
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
