import { proxyGet, proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einladungen ins Träger-Büro durch das moto-Team: offene Einladungen lesen
// (#5749) und eine Person mit einer oder mehreren Rollen einladen (#5144),
// dieselbe Einladung wie aus dem Träger-Portal.
export const GET = proxyGet(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members/invitations`,
);

export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members/invitations`,
);
