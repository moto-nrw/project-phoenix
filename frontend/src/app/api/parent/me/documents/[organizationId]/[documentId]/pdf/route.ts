import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Dokumentenpostfach (#4671): ein Dokument des Trägers als gespeichertes PDF.
//
// Eigener Handler statt des JSON-Proxys, weil hier Bytes durchlaufen. Spiegelt
// den Download-Proxy des Erklärungsnachweises: JWT serverseitig setzen, bei 401
// einmal erneuern, Body mit Content-Type und Content-Disposition unverändert
// durchreichen. Ein Dokument außerhalb des Postfachs beantwortet das Backend
// mit 404, ein Konto ohne Recht mit 403.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { parentAuth, uncachedParentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "ParentDocumentPdf" });

async function GETHandler(
  request: NextRequest,
  {
    params,
  }: { params: Promise<{ organizationId: string; documentId: string }> },
) {
  const { organizationId, documentId } = await params;
  if (!/^\d+$/.test(organizationId) || !/^\d+$/.test(documentId)) {
    return problemResponse(400, "invalid document", request);
  }

  const session = await parentAuth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/parent/me/documents/${organizationId}/${documentId}/pdf`;

  const makeRequest = async (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${bearer}`,
        ...clientIdentityHeaders(request.headers),
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
    logger.warn("parent_document_pdf_proxy_non_ok", {
      document_id: documentId,
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
