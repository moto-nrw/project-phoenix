// Authenticated proxy streaming staff document bytes (#1424). Mirrors the
// student-photo serve proxy: inject the JWT server-side, refresh once on
// 401, stream the body through with Content-Type and Content-Disposition
// intact. Documents are served with `private, no-store`, so there is no
// conditional-GET path to preserve.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffDocumentDownloadRoute" });

async function GETHandler(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string; documentId: string }> },
) {
  const { id, documentId } = await params;

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const backendUrl = `${getServerApiUrl()}/api/staff/${encodeURIComponent(id)}/documents/${encodeURIComponent(documentId)}/download`;

  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: { Authorization: `Bearer ${bearer}` },
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("staff_document_download_proxy_non_ok", {
      staff_id: id,
      document_id: documentId,
      status: upstream.status,
    });
    return new NextResponse(null, { status: upstream.status });
  }

  const headers = new Headers();
  for (const name of [
    "content-type",
    "content-disposition",
    "cache-control",
    "content-length",
  ]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }

  return new NextResponse(upstream.body, { status: 200, headers });
}

export const GET = withTenantAuth(GETHandler);
