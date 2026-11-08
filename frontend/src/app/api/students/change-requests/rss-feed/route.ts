import { proxyGet, proxyPost } from "~/lib/route-wrapper.server";

// The backend answers the feed status and link without an envelope.
export const GET = proxyGet("/api/students/change-requests/rss-feed", {
  raw: true,
});

export const POST = proxyPost("/api/students/change-requests/rss-feed", {
  raw: true,
});
