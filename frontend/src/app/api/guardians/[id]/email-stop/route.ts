import { proxyDelete, proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST stops mail to the person; DELETE takes the stop back (#5934).
export const POST = proxyPost(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/email-stop`,
);
export const DELETE = proxyDelete(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/email-stop`,
);
