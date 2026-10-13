import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

/** The opaque display header, not a browser session, grants backend access. */
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request) =>
    request.headers.get("x-display-token")
      ? "/api/display/dashboard"
      : problemResponse(400, "token required", request),
  cache: "no-store",
  contentTypeOnGet: false,
  additionalHeaders: (request) => ({
    "X-Display-Token": request.headers.get("x-display-token") ?? "",
  }),
});
