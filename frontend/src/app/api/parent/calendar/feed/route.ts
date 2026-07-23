import { proxyGet } from "~/lib/parent/route-wrapper.server";

// GET /api/parent/calendar/feed → backend /parent/me/calendar/feed.
// Returns the parent's iCalendar subscription URLs (generating the token on
// first request).
export const GET = proxyGet<unknown>("/parent/me/calendar/feed");
