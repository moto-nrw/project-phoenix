import { NextResponse } from "next/server";

// The headers of a proxied file download that must reach the browser.
const DOWNLOAD_HEADERS = [
  "content-type",
  "content-disposition",
  "cache-control",
  "content-length",
  "x-content-type-options",
] as const;

/**
 * Passes a backend file download through unchanged: the body as a stream
 * and the headers the browser needs to save it.
 */
export function pdfDownloadResponse(upstream: Response): NextResponse {
  const headers = new Headers();
  for (const name of DOWNLOAD_HEADERS) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new NextResponse(upstream.body, { status: 200, headers });
}
