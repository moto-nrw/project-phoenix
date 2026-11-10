// Kinder in der Familie über den einmaligen Link (#6335): lesen und angeben,
// solange der Link offen ist. Der Link bleibt dabei offen.

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { isCareContractSigningLink } from "~/lib/care-contract-signing-link";

const path = (
  request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const link = params.link;
  if (typeof link !== "string" || !isCareContractSigningLink(link)) {
    return problemResponse(
      410,
      "signing link is not valid",
      { code: "care_contract.signing_link_invalid" },
      request,
    );
  }
  return `/public/care-contract-signing/${encodeURIComponent(link)}/family-children`;
};

export const GET = createPublicJsonProxy({ method: "GET", path });

export const PUT = createPublicJsonProxy({ method: "PUT", path });
