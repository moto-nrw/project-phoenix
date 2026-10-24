import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Rückmeldung der OGS zu den Hausaufgaben eines Kindes in der Lernzeit (#4796).
export const PUT = proxyPut(
  (params) =>
    `/api/timetable/operations/instances/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}/homework-feedback`,
);
