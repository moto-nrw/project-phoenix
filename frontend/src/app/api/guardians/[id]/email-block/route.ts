import { proxyDelete, proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST stops mail to the guardian's address; DELETE frees it again (#5934). The
// backend reads the address from the guardian, never from the request.
export const POST = proxyPost(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/email-block`,
);
export const DELETE = proxyDelete(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/email-block`,
);
