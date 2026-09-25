import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

/** Resolve a tenant without attaching a school session. */
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request) => {
    const slug = request.nextUrl.searchParams.get("slug");
    if (!slug) {
      return NextResponse.json(
        { error: "slug parameter is required" },
        { status: 400 },
      );
    }
    return `/auth/tenant/resolve?slug=${encodeURIComponent(slug)}`;
  },
  contentTypeOnGet: false,
});
