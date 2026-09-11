import { proxyPost } from "~/lib/route-proxy.server";

interface EndSubstitutionRequest {
  type: "group_handover";
  id: number;
}

export const POST = proxyPost<{ ended: boolean }, EndSubstitutionRequest>(
  "/api/substitutions/end",
);
