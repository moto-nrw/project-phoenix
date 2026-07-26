// app/api/timetable/student/[id]/week/route.ts
//
// GET /api/timetable/student/{id}/week?from=YYYY-MM-DD&to=YYYY-MM-DD
//   Proxies the per-child derived care plan across an inclusive date range
//   (backend handler backend/api/timetable/student_day.go → getStudentWeek,
//   max 14 days). Returns one day entry per date, each with arrival/pickup,
//   planned instances + attendance, and any is_unplanned visits.
//
// proxyGet forwards the ?from=&to= query string and strips the backend
// { status, data, message } envelope so route-wrapper does not double-wrap.
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) => `/api/timetable/student/${requirePathSegmentParam(params)}/week`,
);
