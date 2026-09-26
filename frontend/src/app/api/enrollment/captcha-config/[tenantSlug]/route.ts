import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (_request, params) => {
    const tenantSlug = params.tenantSlug;
    if (typeof tenantSlug !== "string" || !tenantSlug) {
      return NextResponse.json(
        { error: "tenant slug is required" },
        { status: 400 },
      );
    }
    return `/api/enrollment/captcha-config/${encodeURIComponent(tenantSlug)}`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
