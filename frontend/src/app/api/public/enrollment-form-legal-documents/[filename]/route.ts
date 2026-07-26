import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";

interface RouteContext {
  params: Promise<{ filename: string }>;
}

export async function GET(_request: NextRequest, context: RouteContext) {
  const { filename } = await context.params;
  const backendUrl = `${getServerApiUrl()}/public/enrollment-form-legal-documents/${encodeURIComponent(filename)}`;
  const response = await fetch(backendUrl, { cache: "no-store" });

  if (!response.ok || !response.body) {
    return NextResponse.json({ error: "Not found" }, { status: 404 });
  }

  return new NextResponse(response.body, {
    status: response.status,
    headers: {
      "content-type": response.headers.get("content-type") ?? "application/pdf",
      "cache-control":
        response.headers.get("cache-control") ?? "public, max-age=86400",
    },
  });
}
