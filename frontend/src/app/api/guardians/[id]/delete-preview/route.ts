import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/guardians/[id]/delete-preview - Read-only blast radius of a full
// delete (admin-only on the backend). Lets the confirmation modal show the
// affected children without the old destructive "probe DELETE" (#819).
export const GET = proxyGet(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/delete-preview`,
);
