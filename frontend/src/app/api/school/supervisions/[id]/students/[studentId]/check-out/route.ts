import { proxyPost } from "~/lib/school/route-wrapper.server";

// Kind der eigenen Aufsicht auschecken (#2527).
export const POST = proxyPost(
  (params) =>
    `/school/supervisions/${params.id as string}/students/${params.studentId as string}/check-out`,
);
