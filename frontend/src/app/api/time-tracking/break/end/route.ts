import { proxyPost } from "~/lib/route-wrapper.server";

/** POST /api/time-tracking/break/end — end the current active break. */
export const POST = proxyPost("/api/time-tracking/break/end");
