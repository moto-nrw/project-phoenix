import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/guardians/[id]/payment/reveal - unmasked IBAN; the backend writes
// a GDPR access-log row for every call, which is why this is a POST.
export const POST = proxyPost(
  (p) => `/api/guardians/${requirePathSegmentParam(p, "id")}/payment/reveal`,
);
