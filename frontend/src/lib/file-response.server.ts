import { problemResponse } from "./api-response-error.server";
import {
  forwardBackendResponse,
  forwardedBody,
} from "./backend-proxy-response.server";

export interface FileResponseOptions {
  /** Keep the backend's Cache-Control, for a versioned image like a logo. */
  backendCacheControl?: boolean;
  /** The BFF's own Cache-Control for an image the browser may keep. */
  cacheControl?: string;
  /** Used when the backend names no type or disposition. */
  fallbackType?: string;
  fallbackDisposition?: string;
}

/**
 * The file headers the browser receives. This proxy is all the browser
 * talks to, so a backend header missing here does not exist in production.
 * Content-Length passes because the backend's bytes pass unchanged (the BFF
 * reaches it uncompressed).
 */
const FILE_HEADERS = [
  "Content-Type",
  "Content-Disposition",
  "Content-Length",
  "X-Content-Type-Options",
] as const;

/**
 * The one answer of every download, export, PDF, ICS and Statistik file: the
 * backend's bytes with their file headers, or the backend's error as its
 * problem body. Never cached unless the caller names a cache policy. The
 * backend's CSP does not travel: the proxy's CSP is the one the browser gets,
 * the sandbox for an uploaded file included (SANDBOXED_FILE_ROUTES).
 */
export async function fileResponse(
  backend: Response,
  request: Request,
  options: FileResponseOptions = {},
): Promise<Response> {
  if (!backend.ok) {
    const response = forwardBackendResponse(backend, request);
    response.headers.set("Cache-Control", "no-store");
    return response;
  }
  if (!backend.body) {
    return problemResponse(502, "No response body from backend", request);
  }
  const headers = new Headers();
  for (const name of FILE_HEADERS) {
    const value = backend.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (options.fallbackType && !headers.has("Content-Type")) {
    headers.set("Content-Type", options.fallbackType);
  }
  if (options.fallbackDisposition && !headers.has("Content-Disposition")) {
    headers.set("Content-Disposition", options.fallbackDisposition);
  }
  if (!headers.has("X-Content-Type-Options")) {
    headers.set("X-Content-Type-Options", "nosniff");
  }
  headers.set(
    "Cache-Control",
    options.cacheControl ??
      ((options.backendCacheControl && backend.headers.get("Cache-Control")) ||
        "no-store"),
  );
  return new Response(forwardedBody(backend), {
    status: backend.status,
    headers,
  });
}

/** An appointment file, for a backend answer that names no type. */
export const ICS_FILE = {
  fallbackType: "text/calendar; charset=utf-8",
  fallbackDisposition: 'attachment; filename="termin.ics"',
} as const satisfies FileResponseOptions;
