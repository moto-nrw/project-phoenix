import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request) => {
    const phase = request.nextUrl.searchParams.get("phase_id");
    if (!phase) {
      return NextResponse.json(
        { error: "phase_id is required" },
        { status: 400 },
      );
    }
    return `/api/enrollment/care-offerings/booking-stats?phase_id=${encodeURIComponent(phase)}`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
