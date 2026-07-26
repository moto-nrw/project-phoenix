// app/api/timetable/instances/edited-in-window/route.ts
//
// GET /api/timetable/instances/edited-in-window
//     ?activity_group_id=ID&from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Proxies the #1875 probe (backend edited_in_window.go): which planned
// occurrences of a template were individually edited in the window, so the
// planner can warn before a series re-plan discards them. Read-only.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/instances/edited-in-window");
