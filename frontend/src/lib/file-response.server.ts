import { problemResponse } from "./api-response-error.server";
import { forwardBackendResponse } from "./backend-proxy-response.server";

export interface FileResponseOptions {
  /**
   * A file a person uploaded, which the browser may show inline: the
   * backend's sandbox CSP and nosniff travel. The proxy sets the same
   * sandbox for these routes (SANDBOXED_FILE_ROUTES in proxy.ts).
   */
  uploadedFile?: boolean;
  /** Keep the backend's Cache-Control, for a versioned image like a logo. */
  backendCacheControl?: boolean;
  /** The BFF's own Cache-Control for an image the browser may keep. */
  cacheControl?: string;
  /**
   * Read the whole file first and send its Content-Length, for a backend
   * that streams without one. Otherwise the bytes stream through.
   */
  buffer?: boolean;
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
 * problem body. Never cached unless the caller names a cache policy.
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
  const csp = backend.headers.get("Content-Security-Policy");
  if (options.uploadedFile && csp) headers.set("Content-Security-Policy", csp);
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
  if (!options.buffer) {
    return new Response(backend.body, { status: backend.status, headers });
  }
  const bytes = await backend.arrayBuffer();
  headers.set("Content-Length", String(bytes.byteLength));
  return new Response(bytes, { status: backend.status, headers });
}

/** An appointment file, for a backend answer that names no type. */
export const ICS_FILE = {
  fallbackType: "text/calendar; charset=utf-8",
  fallbackDisposition: 'attachment; filename="termin.ics"',
} as const satisfies FileResponseOptions;
