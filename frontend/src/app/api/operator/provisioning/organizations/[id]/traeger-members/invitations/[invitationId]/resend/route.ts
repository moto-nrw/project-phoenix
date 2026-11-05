import { proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einladung ins Träger-Büro erneut senden (#5749): neuer Link, neue
// Gültigkeit, der alte Link funktioniert nicht mehr.
export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members/invitations/${requirePathSegmentParam(params, "invitationId")}/resend`,
);
