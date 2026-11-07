import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import type { NextRequest } from "next/server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { createLogger } from "~/lib/logger";
import { getServerApiUrl } from "~/lib/server-api-url";
import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import {
  operatorAuth,
  uncachedOperatorAuth,
  withOperatorAuth,
} from "~/server/auth/operator";

const logger = createLogger({ component: "OperatorBillingExportRoute" });

/**
 * Streams the billing key-date CSV (#2791) from the backend as a download.
 * It bypasses the JSON route wrapper because the body is a file with its own
 * Content-Disposition, not an envelope.
 */
function exportQuery(request: NextRequest): string {
  const month = request.nextUrl.searchParams.get("month");
  return month ? `?month=${encodeURIComponent(month)}` : "";
}

function exportResponse(
  backendResponse: Response,
  request: NextRequest,
): Response {
  if (!backendResponse.ok) {
    const response = forwardBackendResponse(backendResponse, request);
    response.headers.set("Cache-Control", "no-store");
    return response;
  }
  if (!backendResponse.body) {
    return operatorErrorResponse(
      502,
      "general.unavailable",
      "No response body from backend",
    );
  }

  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const name of [
    "Content-Type",
    "Content-Disposition",
    "Content-Length",
  ]) {
    const value = backendResponse.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(backendResponse.body, {
    status: backendResponse.status,
    headers,
  });
}

async function GETHandler(request: NextRequest) {
  try {
    const session = await operatorAuth();
    if (!session?.user?.token) {
      return operatorErrorResponse(401, "general.permission", "Unauthorized");
    }

    const url = `${getServerApiUrl()}/operator/billing/key-date-counts/export${exportQuery(request)}`;
    const send = (bearer: string) =>
      fetch(url, {
        headers: { Authorization: `Bearer ${bearer}` },
        cache: "no-store",
      });
    return exportResponse(
      await sendWithTokenRefresh(session.user.token, send, () =>
        uncachedOperatorAuth(),
      ),
      request,
    );
  } catch (error) {
    captureBffException(error, request);
    logger.error("operator billing export proxy failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return operatorErrorResponse(
      503,
      "general.unavailable",
      "Export proxy failed",
    );
  }
}

export const GET = withPrivateDownloadNoStore(withOperatorAuth(GETHandler));
