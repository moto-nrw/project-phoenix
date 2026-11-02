import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
// Authenticated proxy streaming one proof file of an enrollment request
// (#4783). Mirrors the child document download proxy: inject the JWT
// server-side, refresh once on 401, stream the bytes with their headers.
// The backend checks enrollment_uploads:read, audits every call and sends
// the file inline with a sandbox CSP and `no-store`; those headers must
// reach the browser, because this proxy is all the browser talks to.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "EnrollmentUploadDownloadRoute" });

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; uploadId: string }> },
) {
  const { id, uploadId } = await params;

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/api/enrollment/admin/requests/${encodeURIComponent(id)}/uploads/${encodeURIComponent(uploadId)}`;

  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: { Authorization: `Bearer ${bearer}` },
      cache: "no-store",
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("enrollment_upload_download_proxy_non_ok", {
      request_id: id,
      upload_id: uploadId,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }

  const headers = new Headers();
  for (const name of [
    "content-type",
    "content-disposition",
    "content-length",
    "content-security-policy",
    "x-content-type-options",
  ]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  // Income proofs never belong in a shared or browser cache.
  headers.set("cache-control", "no-store");
  if (!headers.has("x-content-type-options")) {
    headers.set("x-content-type-options", "nosniff");
  }

  return new NextResponse(upstream.body, { status: 200, headers });
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
