import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

/** The opaque display header, not a browser session, grants backend access. */
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request) =>
    request.headers.get("x-display-token")
      ? "/api/display/dashboard"
      : NextResponse.json({ error: "token required" }, { status: 400 }),
  cache: "no-store",
  contentTypeOnGet: false,
  additionalHeaders: (request) => ({
    "X-Display-Token": request.headers.get("x-display-token") ?? "",
  }),
});
