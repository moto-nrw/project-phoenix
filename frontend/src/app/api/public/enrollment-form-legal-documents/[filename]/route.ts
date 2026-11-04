import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

interface RouteContext {
  params: Promise<{ filename: string }>;
}

async function GETHandler(_request: NextRequest, context: RouteContext) {
  const { filename } = await context.params;
  const backendUrl = `${getServerApiUrl()}/public/enrollment-form-legal-documents/${encodeURIComponent(filename)}`;
  const response = await fetch(backendUrl, {
    headers: clientIdentityHeaders(_request.headers),
    cache: "no-store",
  });

  if (!response.ok) return forwardBackendResponse(response, _request);
  if (!response.body) return problemResponse(404, "Not found", _request);

  return new NextResponse(response.body, {
    status: response.status,
    headers: {
      "content-type": response.headers.get("content-type") ?? "application/pdf",
      "cache-control":
        response.headers.get("cache-control") ?? "public, max-age=86400",
    },
  });
}

export const GET = withPrivateNoStore(GETHandler);
