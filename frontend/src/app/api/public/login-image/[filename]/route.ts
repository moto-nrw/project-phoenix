import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { sendBackend } from "~/lib/backend-client.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

// Public endpoint — no authentication required.
// Serves tenant login images stored on the backend.
async function GETHandler(
  _request: NextRequest,
  context: { params: Promise<{ filename: string }> },
) {
  const { filename } = await context.params;

  // Path traversal protection
  if (!filename || filename.includes("..") || filename.includes("/")) {
    return new NextResponse(null, { status: 404 });
  }

  const backendPath = `/public/login-image/${encodeURIComponent(filename)}`;

  let response: Response;
  try {
    response = await sendBackend(backendPath, null, {
      request: _request,
      timeout: "publicAsset",
    });
  } catch {
    // Network error, DNS failure, or timeout — return 502 instead of
    // letting the error bubble up as an unhandled 500.
    return new NextResponse(null, { status: 502 });
  }

  if (!response.ok) {
    return forwardBackendResponse(response, _request);
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

export const GET = withPrivateNoStore(GETHandler);
