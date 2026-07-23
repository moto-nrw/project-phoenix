import { proxyPost } from "~/lib/parent/route-wrapper.server";

// POST /api/parent/calendar/feed/rotate → backend
// /parent/me/calendar/feed/rotate. Issues a fresh subscription token,
// invalidating the previous URL.
export const POST = proxyPost<unknown>("/parent/me/calendar/feed/rotate");
