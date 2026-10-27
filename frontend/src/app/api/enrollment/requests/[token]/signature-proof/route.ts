// Public proxy for the parent's proof of a signed enrollment (#4786). The
// status token is the only authentication; the backend resolves it to one
// request of one school and answers 404 for an unknown, expired or altered
// token. The bytes and their headers pass through unchanged. The client
// identity goes along so the backend rate-limits and audits the family, not
// the frontend container.

import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { createLogger } from "~/lib/logger";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { pdfDownloadResponse } from "~/lib/pdf-download-response.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

const logger = createLogger({
  component: "EnrollmentStatusSignatureProofRoute",
});

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ token: string }> },
) {
  const { token } = await params;
  if (!token) {
    return problemResponse(400, "token required", request);
  }
  const upstream = await fetch(
    `${getServerApiUrl()}/api/enrollment/requests/${encodeURIComponent(token)}?format=pdf`,
    {
      method: "GET",
      headers: clientIdentityHeaders(request.headers),
      cache: "no-store",
    },
  );
  if (!upstream.ok) {
    logger.warn("enrollment_status_signature_proof_non_ok", {
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }
  return pdfDownloadResponse(upstream);
}

export const GET = withPrivateNoStore(GETHandler);
