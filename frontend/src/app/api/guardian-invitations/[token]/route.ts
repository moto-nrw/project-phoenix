import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (_request, params) => {
    const token = params.token;
    if (typeof token !== "string" || !token) {
      return problemResponse(400, "Missing invitation token", _request);
    }
    return `/auth/guardian-invitations/${encodeURIComponent(token)}`;
  },
  contentTypeOnGet: false,
});
