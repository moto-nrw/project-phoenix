import { type NextRequest, NextResponse } from "next/server";
import { sendBackend } from "~/lib/backend-client.server";
import {
  forwardBackendResponse,
  forwardedBody,
} from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { handleApiError } from "~/lib/api-response-error.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

interface RouteContext {
  params: Promise<{ filename: string }>;
}

async function GETHandler(_request: NextRequest, context: RouteContext) {
  const { filename } = await context.params;
  let response: Response;
  try {
    response = await sendBackend(
      `/public/enrollment-form-legal-documents/${encodeURIComponent(filename)}`,
      null,
      { request: _request },
    );
  } catch (error) {
    return handleApiError(error, _request);
  }

  if (!response.ok) return forwardBackendResponse(response, _request);
  if (!response.body) return problemResponse(404, "Not found", _request);

  return new NextResponse(forwardedBody(response), {
    status: response.status,
    headers: {
      "content-type": response.headers.get("content-type") ?? "application/pdf",
      "cache-control":
        response.headers.get("cache-control") ?? "public, max-age=86400",
    },
  });
}

export const GET = withPrivateNoStore(GETHandler);
