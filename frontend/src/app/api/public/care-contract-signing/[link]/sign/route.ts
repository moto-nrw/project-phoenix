// Unterschrift über den einmaligen Link (#4679): der einzige Aufruf, der den
// Link schließt.

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { isCareContractSigningLink } from "~/lib/care-contract-signing-link";

export const POST = createPublicJsonProxy({
  method: "POST",
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
    return `/public/care-contract-signing/${encodeURIComponent(link)}/sign`;
  },
});
