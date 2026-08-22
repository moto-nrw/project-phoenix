import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/pickup-schedules/reset-offering — setzt die
// Gehzeit eines Wochentags auf die Angebots-Gehzeit zurück (#2290)
export const POST = proxyPost(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/pickup-schedules/reset-offering`,
);
