import { proxyPatch } from "~/lib/school/route-wrapper.server";

// Anwesenheitsstatus eines Kindes der eigenen Aufsicht setzen (#2527).
export const PATCH = proxyPatch(
  (params) =>
    `/school/supervisions/${encodeURIComponent(params.id as string)}/students/${encodeURIComponent(params.studentId as string)}/attendance`,
);
