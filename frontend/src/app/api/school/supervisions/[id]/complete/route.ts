import { proxyPost } from "~/lib/school/route-wrapper.server";

// Eigene Aufsicht beenden (#2527).
export const POST = proxyPost(
  (params) => `/school/supervisions/${params.id as string}/complete`,
);
