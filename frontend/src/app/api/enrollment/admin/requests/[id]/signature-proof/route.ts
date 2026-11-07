import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
// Authenticated proxy for the staff proof of a signed enrollment (#4786):
// the backend answers the request detail with ?format=pdf as a PDF with the
// parent's signature.

import type { NextRequest } from "next/server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";
import { pdfDownloadResponse } from "~/lib/pdf-download-response.server";

const logger = createLogger({ component: "EnrollmentSignatureProofRoute" });

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  if (!/^\d+$/.test(id)) {
    return problemResponse(400, "invalid id", request);
  }
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/api/enrollment/admin/requests/${encodeURIComponent(id)}?format=pdf`;
  const makeRequest = async (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${bearer}`,
        ...(await incomingAnalyticsSessionHeaders()),
      },
      cache: "no-store",
    });

  const upstream = await sendWithTokenRefresh(token, makeRequest, () =>
    uncachedAuth(),
  );
  if (!upstream.ok) {
    logger.warn("enrollment_signature_proof_proxy_non_ok", {
      request_id: id,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }
  return pdfDownloadResponse(upstream);
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
