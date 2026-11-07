import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import type { NextRequest } from "next/server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";
import { incomingAnalyticsSessionHeaders } from "~/lib/analytics-session-header.server";

// Wochenprotokoll (#4769): das PDF eines abgeschlossenen Protokolls. Das
// Backend prüft die Reichweite der Person und schreibt vor der Auslieferung
// den Eintrag ins Zugriffsprotokoll; 403/404/409 laufen unverändert durch.

const logger = createLogger({ component: "WeekProtocolPdfRoute" });

interface RouteContext {
  params: Promise<{ protocolId: string }>;
}

function pdfResponse(
  backendResponse: Response,
  request: NextRequest,
): Response {
  if (!backendResponse.ok) {
    const response = forwardBackendResponse(backendResponse, request);
    response.headers.set("Cache-Control", "no-store");
    return response;
  }
  if (!backendResponse.body) {
    return problemResponse(502, "No response body from backend", request);
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

async function GETHandler(request: NextRequest, context: RouteContext) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }
    const { protocolId } = await context.params;
    if (!/^\d+$/.test(protocolId)) {
      return problemResponse(400, "invalid id", request);
    }
    const url = `${getServerApiUrl()}/api/week-protocols/${encodeURIComponent(protocolId)}/pdf`;
    const forwardHeaders = await incomingAnalyticsSessionHeaders();
    const send = (bearer: string) =>
      fetch(url, {
        headers: { Authorization: `Bearer ${bearer}`, ...forwardHeaders },
        cache: "no-store",
      });
    return pdfResponse(
      await sendWithTokenRefresh(session.user.token, send, () =>
        uncachedAuth(),
      ),
      request,
    );
  } catch (error) {
    captureBffException(error, request);
    logger.error("week_protocol_pdf_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
