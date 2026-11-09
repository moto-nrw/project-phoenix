import { proxyGet } from "~/lib/route-wrapper.server";

// Tagesauswertung (#1456). The backend checks gdpr.attendance_log_enabled, the
// group scope and the visibility cap, and audits exactly what it was asked.
export const GET = proxyGet("/api/students/day-log", {
  forwardQuery: ["date", "group_id"],
});
