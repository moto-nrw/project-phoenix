import { proxyPost } from "~/lib/school/route-wrapper.server";

// Eigene Aufsicht starten (#2527).
export const POST = proxyPost(
  (params) =>
    `/school/supervisions/${encodeURIComponent(params.id as string)}/start`,
);
