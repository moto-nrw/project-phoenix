import { NextResponse } from "next/server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { tenant } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = tenant.route(async (request, _token, params) => {
  // The backend marks each per-upload photo URL as privately cacheable
  // and immutable, so browsers normally reuse their copy without asking.
  // When they do revalidate (expired entry, forced reload), forward the
  // conditional headers so http.ServeContent can answer 304 instead of
  // re-sending the full body.
  const conditional: Record<string, string> = {};
  for (const name of ["if-none-match", "if-modified-since"]) {
    const value = request.headers.get(name);
    if (value) conditional[name] = value;
  }
  const upstream = await tenant.backendFetch(
    request,
    `/api/students/${requirePathSegmentParam(params)}/photo/${requirePathSegmentParam(params, "filename")}`,
    { headers: conditional },
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

  if (!upstream.ok) return forwardBackendResponse(upstream, request);

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
});
