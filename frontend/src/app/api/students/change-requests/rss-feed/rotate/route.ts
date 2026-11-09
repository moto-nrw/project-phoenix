import { proxyPost } from "~/lib/route-wrapper.server";

// The backend answers the new link without an envelope.
export const POST = proxyPost("/api/students/change-requests/rss-feed/rotate", {
  raw: true,
});
