import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "StudentPhotoServeRoute" });

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; filename: string }> },
) {
  const { id, filename } = await params;

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthorized", request);
  }

  const backendUrl = `${getServerApiUrl()}/api/students/${encodeURIComponent(id)}/photo/${encodeURIComponent(filename)}`;

  // The backend marks each per-upload photo URL as privately cacheable
  // and immutable, so browsers normally reuse their copy without asking.
  // When they do revalidate (expired entry, forced reload), forward the
  // conditional headers so http.ServeContent can answer 304 instead of
  // re-sending the full body.
  const buildHeaders = (bearer: string): HeadersInit => {
    const h: Record<string, string> = {
      Authorization: `Bearer ${bearer}`,
      ...clientIdentityHeaders(request.headers),
    };
    const ifNoneMatch = request.headers.get("if-none-match");
    if (ifNoneMatch) h["if-none-match"] = ifNoneMatch;
    const ifModifiedSince = request.headers.get("if-modified-since");
    if (ifModifiedSince) h["if-modified-since"] = ifModifiedSince;
    return h;
  };

  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: buildHeaders(bearer),
    });

  const upstream = await sendWithTokenRefresh(token, makeRequest, () =>
    uncachedAuth(),
  );

  // 304 keeps the browser on its cached copy.
  if (upstream.status === 304) {
    const headers = new Headers();
    const cacheControl = upstream.headers.get("cache-control");
    if (cacheControl) headers.set("cache-control", cacheControl);
    const etag = upstream.headers.get("etag");
    if (etag) headers.set("etag", etag);
    const lastModified = upstream.headers.get("last-modified");
    if (lastModified) headers.set("last-modified", lastModified);
    return new NextResponse(null, { status: 304, headers });
  }

  if (!upstream.ok) {
    logger.warn("student_photo_serve_proxy_non_ok", {
      student_id: id,
      filename,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream, request);
  }

  // The validators let a later conditional GET end in the 304 above.
  const headers = new Headers();
  const contentType = upstream.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);
  const cacheControl = upstream.headers.get("cache-control");
  if (cacheControl) headers.set("cache-control", cacheControl);
  const etag = upstream.headers.get("etag");
  if (etag) headers.set("etag", etag);
  const lastModified = upstream.headers.get("last-modified");
  if (lastModified) headers.set("last-modified", lastModified);

  return new NextResponse(upstream.body, {
    status: 200,
    headers,
  });
}

export const GET = withTenantAuth(GETHandler);
