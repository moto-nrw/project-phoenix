// app/api/timetable/student/[id]/day/route.ts
//
// GET /api/timetable/student/{id}/day?date=YYYY-MM-DD
//   Proxies the per-child derived care plan for a single day (backend handler
//   backend/api/timetable/student_day.go → getStudentDay). The response merges
//   the resolved arrival/pickup slot, the planned activity instances with the
//   child's attendance, and any is_unplanned visits.
//
// proxyGet forwards the ?date= query string and strips the backend
// { status, data, message } envelope so route-wrapper does not double-wrap.
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) => `/api/timetable/student/${requirePathSegmentParam(params)}/day`,
);
