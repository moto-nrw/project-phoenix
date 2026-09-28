import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request, params) => {
    const { tenantSlug, phaseId } = params;
    if (
      typeof tenantSlug !== "string" ||
      !tenantSlug ||
      typeof phaseId !== "string" ||
      !phaseId
    ) {
      return NextResponse.json(
        { error: "tenant slug and phaseId are required" },
        { status: 400 },
      );
    }
    const path = `/api/enrollment/care-offerings/public/${encodeURIComponent(tenantSlug)}/${encodeURIComponent(phaseId)}`;
    const lateInvite = request.nextUrl.searchParams.get("late_invite")?.trim();
    return lateInvite
      ? `${path}?late_invite=${encodeURIComponent(lateInvite)}`
      : path;
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
