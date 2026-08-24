import { proxyPatch } from "~/lib/school/route-wrapper.server";

// Anwesenheitsstatus eines Kindes der eigenen Aufsicht setzen (#2527).
export const PATCH = proxyPatch(
  (params) =>
    `/school/supervisions/${params.id as string}/students/${params.studentId as string}/attendance`,
);
