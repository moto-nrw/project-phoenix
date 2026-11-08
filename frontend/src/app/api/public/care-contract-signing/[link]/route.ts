// Öffentliche Seite eines Betreuungsvertrags hinter dem einmaligen Link aus
// der Mail (#4679). Der Link ist die einzige Berechtigung. Lesen verbraucht
// ihn nicht; erst die Unterschrift schließt ihn.

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { isCareContractSigningLink } from "~/lib/care-contract-signing-link";

export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request, params) => {
    const link = params.link;
    if (typeof link !== "string" || !isCareContractSigningLink(link)) {
      return problemResponse(
        410,
        "signing link is not valid",
        { code: "care_contract.signing_link_invalid" },
        request,
      );
    }
    return `/public/care-contract-signing/${encodeURIComponent(link)}`;
  },
});
