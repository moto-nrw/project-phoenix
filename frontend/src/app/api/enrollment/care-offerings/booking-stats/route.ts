import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request) => {
    const phase = request.nextUrl.searchParams.get("phase_id");
    if (!phase) {
      return problemResponse(400, "phase_id is required", request);
    }
    return `/api/enrollment/care-offerings/booking-stats?phase_id=${encodeURIComponent(phase)}`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
