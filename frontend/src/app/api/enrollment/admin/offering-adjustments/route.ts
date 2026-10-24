import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request) => {
    const requestID = request.nextUrl.searchParams.get("request_id") ?? "";
    const childID = request.nextUrl.searchParams.get("child_id") ?? "";
    if (!requestID || !childID) {
      return problemResponse(
        400,
        "request_id and child_id are required",
        request,
      );
    }
    return `/api/enrollment/admin/requests/${encodeURIComponent(requestID)}/children/${encodeURIComponent(childID)}/offering-adjustments`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
