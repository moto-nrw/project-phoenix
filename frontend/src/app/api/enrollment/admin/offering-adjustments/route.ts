import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request) => {
    const requestID = request.nextUrl.searchParams.get("request_id") ?? "";
    const childID = request.nextUrl.searchParams.get("child_id") ?? "";
    if (!requestID || !childID) {
      return NextResponse.json(
        { error: "request_id and child_id are required" },
        { status: 400 },
      );
    }
    return `/api/enrollment/admin/requests/${encodeURIComponent(requestID)}/children/${encodeURIComponent(childID)}/offering-adjustments`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
