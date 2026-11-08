import { proxyGet } from "~/lib/school/route-wrapper.server";

// Kinderliste einer eigenen Aufsicht (#2527).
export const GET = proxyGet(
  (params) =>
    `/school/supervisions/${encodeURIComponent(params.id as string)}/roster`,
);
