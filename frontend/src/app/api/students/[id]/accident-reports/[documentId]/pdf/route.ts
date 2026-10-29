import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { captureBffException } from "~/lib/sentry-bff.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

// Unfallanzeige (#4771): das PDF im Aufbau des amtlichen Vordrucks. Das
// Backend prüft student_accidents:read und schreibt vor der Auslieferung den
// Eintrag ins Zugriffsprotokoll; 403/404 laufen unverändert durch.

const logger = createLogger({ component: "AccidentReportPdfRoute" });

interface RouteContext {
  params: Promise<{ id: string; documentId: string }>;
}

async function proxyPdf(
  request: NextRequest,
  studentId: string,
  documentId: string,
  token: string,
): Promise<Response> {
  const backendResponse = await fetch(
    `${getServerApiUrl()}/api/students/${encodeURIComponent(studentId)}/accident-reports/${encodeURIComponent(documentId)}/pdf`,
    { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" },
  );
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
    const { id, documentId } = await context.params;
    if (!/^\d+$/.test(id) || !/^\d+$/.test(documentId)) {
      return problemResponse(400, "invalid id", request);
    }
    const response = await proxyPdf(
      request,
      id,
      documentId,
      session.user.token,
    );
    if (response.status !== 401) return response;
    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyPdf(request, id, documentId, refreshed.user.token);
  } catch (error) {
    captureBffException(error, request);
    logger.error("accident_report_pdf_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal server error", request);
  }
}

export const GET = withPrivateDownloadNoStore(withTenantAuth(GETHandler));
