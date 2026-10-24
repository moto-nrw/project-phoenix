import { proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Rückmeldung zu den Hausaufgaben eines Kindes der eigenen Lernzeit (#4796).
export const PUT = proxyPut(
  (params) =>
    `/school/supervisions/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}/homework-feedback`,
);
