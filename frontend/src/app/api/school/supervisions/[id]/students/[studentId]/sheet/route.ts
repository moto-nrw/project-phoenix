import { proxyGet } from "~/lib/school/route-wrapper.server";

// Abhol- und Notfallinformationen eines Kindes der eigenen Aufsicht (#2527).
// Jeder Aufruf wird serverseitig protokolliert.
export const GET = proxyGet(
  (params) =>
    `/school/supervisions/${params.id as string}/students/${params.studentId as string}/sheet`,
);
