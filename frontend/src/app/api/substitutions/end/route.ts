import { proxyPost } from "~/lib/route-proxy.server";

// "additional_supervision" ends the caller's own part in the running
// supervision whose id is the active group (#4763).
interface EndSubstitutionRequest {
  type: "group_handover" | "additional_supervision";
  id: number | string;
}

export const POST = proxyPost<{ ended: boolean }, EndSubstitutionRequest>(
  "/api/substitutions/end",
);
