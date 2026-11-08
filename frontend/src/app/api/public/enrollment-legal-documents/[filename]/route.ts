import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { sendBackend } from "~/lib/backend-client.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

async function GETHandler(
  _request: NextRequest,
  context: { params: Promise<{ filename: string }> },
) {
  const { filename } = await context.params;

  if (!filename || filename.includes("..") || filename.includes("/")) {
    return new NextResponse(null, { status: 404 });
  }

  const backendPath = `/public/enrollment-legal-documents/${encodeURIComponent(filename)}`;

  let response: Response;
  try {
    response = await sendBackend(backendPath, null, {
      request: _request,
      timeout: "publicAsset",
    });
  } catch {
    return new NextResponse(null, { status: 502 });
  }

  if (!response.ok) {
    return forwardBackendResponse(response, _request);
  }

  const contentType = response.headers.get("Content-Type") ?? "";
  if (!contentType.startsWith("application/pdf")) {
    return new NextResponse(null, { status: 502 });
  }

  const body = await response.arrayBuffer();

  return new NextResponse(body, {
    headers: {
      "Content-Type": contentType,
      "Cache-Control": "public, max-age=86400",
      "Content-Disposition": `inline; filename="${filename}"`,
    },
  });
}

export const GET = withPrivateNoStore(GETHandler);
