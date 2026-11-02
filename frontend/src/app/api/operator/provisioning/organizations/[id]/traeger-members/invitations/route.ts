import { proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einladung ins Träger-Büro durch das moto-Team (#5144): dieselbe Einladung
// wie aus dem Träger-Portal, mit einer oder mehreren Rollen.
export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members/invitations`,
);
