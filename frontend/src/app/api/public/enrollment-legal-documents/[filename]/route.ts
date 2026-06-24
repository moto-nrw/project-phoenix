import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ filename: string }> },
) {
  const { filename } = await context.params;

  if (!filename || filename.includes("..") || filename.includes("/")) {
    return new NextResponse(null, { status: 404 });
  }

  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const backendUrl = `${getServerApiUrl()}/public/enrollment-legal-documents/${encodeURIComponent(filename)}`;

  let response: Response;
  try {
    response = await fetch(backendUrl, {
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    return new NextResponse(null, { status: 502 });
  }

  if (!response.ok) {
    return new NextResponse(null, {
      status: response.status === 404 ? 404 : 502,
    });
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
