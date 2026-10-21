import { proxyPost } from "~/lib/route-proxy.server";

/** POST /api/time-tracking/break/end — end the current active break. */
export const POST = proxyPost("/api/time-tracking/break/end");
