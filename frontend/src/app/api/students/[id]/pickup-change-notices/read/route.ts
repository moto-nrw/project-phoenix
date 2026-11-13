import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/pickup-change-notices/read - Die geladenen
// Abholzeit-Hinweise dieses Kindes sind gesehen; antwortet mit den noch
// ungesehenen (#6855)
export const POST = proxyPost(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/pickup-change-notices/read`,
);
