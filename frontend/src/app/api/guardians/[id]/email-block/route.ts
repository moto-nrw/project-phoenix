import { proxyDelete } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// DELETE frees an address someone checked (#5934). The backend reads the address
// from the guardian, never from the request.
export const DELETE = proxyDelete(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/email-block`,
);
