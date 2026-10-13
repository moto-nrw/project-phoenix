import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

/** Resolve a tenant without attaching a school session. */
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request) => {
    const slug = request.nextUrl.searchParams.get("slug");
    if (!slug) {
      return problemResponse(400, "slug parameter is required", request);
    }
    return `/auth/tenant/resolve?slug=${encodeURIComponent(slug)}`;
  },
  contentTypeOnGet: false,
});
