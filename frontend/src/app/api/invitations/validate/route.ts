import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request) => {
    const token = request.nextUrl.searchParams.get("token");
    if (!token) {
      return problemResponse(400, "Missing invitation token", request);
    }
    return `/auth/invitations/${encodeURIComponent(token)}`;
  },
  contentTypeOnGet: false,
});
