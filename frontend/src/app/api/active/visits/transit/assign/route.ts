import { proxyPost } from "~/lib/route-proxy.server";

interface AssignTransitBody {
  student_ids: number[];
  active_group_id: number;
}

export const POST = proxyPost<unknown, AssignTransitBody>(
  "/api/active/visits/transit/assign",
);
