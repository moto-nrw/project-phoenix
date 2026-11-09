// Das eingefrorene PDF eines Vertrags hinter dem einmaligen Link (#4679).
// Lesen verbraucht den Link nicht. Bytes und Kopfzeilen gehen unverändert
// durch.

import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { isCareContractSigningLink } from "~/lib/care-contract-signing-link";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";

export const GET = withPrivateDownloadNoStore(
  createPublicJsonProxy({
    method: "GET",
    onSuccess: (response) => {
      const headers = new Headers({
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      });
      for (const name of [
        "Content-Type",
        "Content-Disposition",
        "Content-Length",
      ]) {
        const value = response.headers.get(name);
        if (value) headers.set(name, value);
      }
      return new Response(response.body, { status: response.status, headers });
    },
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
      return `/public/care-contract-signing/${encodeURIComponent(link)}/document`;
    },
  }),
);
