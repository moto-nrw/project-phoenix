// app/api/students/[id]/photo/[filename]/route.ts
//
// Authenticated proxy that streams a student's stored photo bytes back to
// the browser. The backend response shape (StudentResponse.photo_url) points
// at this exact path so `<img src=photoUrl>` Just Works on same-origin —
// the browser sends the NextAuth session cookie, this handler resolves it
// to a JWT, and forwards to the backend's serveStudentPhoto with bearer auth.
//
// Stored DB path stays /uploads/student-photos/{filename} (the backend's
// cleanup + file unlink expects that prefix); only the JSON-facing URL
// the frontend renders is the proxy URL above.
//
// 401 retry: when the cached session token expires (15 min) but the
// NextAuth refresh token is still valid, the first backend call returns
// 401. We then call uncachedAuth() once to force a token refresh and
// retry. Mirrors the staff avatar proxy at api/staff/[id]/avatar/route.ts.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentPhotoServeRoute" });

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; filename: string }> },
) {
  const { id, filename } = await params;

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const backendUrl = `${getServerApiUrl()}/api/students/${encodeURIComponent(id)}/photo/${encodeURIComponent(filename)}`;

  // Forward the browser's revalidation headers so the backend's
  // http.ServeContent path can return 304 Not Modified instead of the
  // full body. Without this, browsers always re-download the bytes even
  // though the backend sets `Cache-Control: private, no-cache` (which
  // means "you may store, but MUST revalidate" — useless if the proxy
  // strips the conditional headers). On pages that render many avatars
  // (search results, room views, supervision lists) this materially
  // increases bandwidth and disk reads on the backend.
  const buildHeaders = (bearer: string): HeadersInit => {
    const h: Record<string, string> = { Authorization: `Bearer ${bearer}` };
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

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  // 304 Not Modified is the success-with-no-body path — preserve it
  // verbatim so the browser keeps using its cached copy. ETag /
  // Last-Modified are passed through (when present) for the same reason.
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
    return new NextResponse(null, { status: upstream.status });
  }

  // 200 path — stream the body through unchanged. Preserve Content-Type,
  // the backend's Cache-Control (private, no-cache so the browser
  // revalidates next time), and the validators (ETag / Last-Modified) so
  // the conditional-GET round-trip on the next render can short-circuit
  // to 304 above without re-downloading the full image.
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
