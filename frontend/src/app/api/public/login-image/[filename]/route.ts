import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Public endpoint — no authentication required.
// Serves tenant login images stored on the backend.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ filename: string }> },
) {
  const { filename } = await context.params;

  // Path traversal protection
  if (!filename || filename.includes("..") || filename.includes("/")) {
    return new NextResponse(null, { status: 404 });
  }

  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const backendUrl = `${getServerApiUrl()}/public/login-image/${encodeURIComponent(filename)}`;

  let response: Response;
  try {
    response = await fetch(backendUrl, {
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    // Network error, DNS failure, or timeout — return 502 instead of
    // letting the error bubble up as an unhandled 500.
    return new NextResponse(null, { status: 502 });
  }

  if (!response.ok) {
    return new NextResponse(null, {
      status: response.status === 404 ? 404 : 502,
    });
  }

  const contentType = response.headers.get("Content-Type") ?? "";
  if (!contentType.startsWith("image/")) {
    return new NextResponse(null, { status: 502 });
  }

  const body = await response.arrayBuffer();

  return new NextResponse(body, {
    headers: {
      "Content-Type": contentType,
      "Cache-Control": "public, max-age=86400",
    },
  });
}
