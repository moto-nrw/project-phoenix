import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Die Datei eines Dokuments, das der Träger für die Schule freigab (#4692).
// Wie der Download der Dateiablage: Token serverseitig, einmal erneuern bei
// 401, Bytes mit Content-Type und Content-Disposition durchreichen. Das
// Backend liefert `private, no-store` und zeichnet das Öffnen auf.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";
import { clientIdentityHeaders } from "~/lib/client-headers.server";

const logger = createLogger({ component: "TraegerDocumentDownloadRoute" });

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/api/traeger-documents/${encodeURIComponent(id)}/file`;
  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: {
        ...clientIdentityHeaders(request.headers),
        Authorization: `Bearer ${bearer}`,
      },
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("traeger_document_download_non_ok", {
      document_id: id,
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

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
