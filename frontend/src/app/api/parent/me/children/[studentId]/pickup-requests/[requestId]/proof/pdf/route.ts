import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Abholerlaubnis (#4780), Elternseite: der Nachweis einer unterschriebenen
// Anfrage als PDF der Schule. Nur die Person, die unterschrieben hat, bekommt
// ihn; sonst antwortet das Backend mit 404.
//
// Eigener Handler statt des JSON-Proxys, weil hier Bytes durchlaufen. Wie der
// Nachweis der Einverständnisse: JWT serverseitig setzen, bei 401 einmal
// erneuern, Body mit Content-Type und Content-Disposition durchreichen.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { parentAuth, uncachedParentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "ParentPickupProofPdf" });

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ studentId: string; requestId: string }> },
) {
  const { studentId, requestId } = await params;
  if (!/^\d+$/.test(studentId) || !/^\d+$/.test(requestId)) {
    return problemResponse(400, "invalid request", request);
  }

  const session = await parentAuth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/parent/me/children/${encodeURIComponent(studentId)}/pickup-requests/${encodeURIComponent(requestId)}/proof?format=pdf`;

  const makeRequest = async (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${bearer}`,
        ...(await incomingAnalyticsSessionHeaders()),
      },
      cache: "no-store",
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedParentAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("parent_pickup_proof_pdf_proxy_non_ok", {
      request_id: requestId,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }

  const headers = new Headers();
  for (const name of [
    "content-type",
    "content-disposition",
    "cache-control",
    "content-length",
    "x-content-type-options",
  ]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }

  return new NextResponse(upstream.body, { status: 200, headers });
}

export const GET = withPrivateDownloadNoStore(withParentAuth(GETHandler));
