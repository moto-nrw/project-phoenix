import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
// Erklärungen (#3430), Elternseite: der Nachweis als PDF der Schule für dieses
// Konto und ein Kind.
//
// Eigener Handler statt des JSON-Proxys, weil hier Bytes durchlaufen. Spiegelt
// den Download-Proxy der Anhänge: JWT serverseitig setzen, bei 401 einmal
// erneuern, Body mit Content-Type und Content-Disposition unverändert
// durchreichen. Ohne Zugriff oder ohne eigene Antwort antwortet das Backend
// mit 404.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { parentAuth, uncachedParentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentDeclarationProofPdf" });

async function GETHandler(
  request: NextRequest,
  { params }: { params: Promise<{ announcementId: string }> },
) {
  const { announcementId } = await params;

  const studentId = request.nextUrl.searchParams.get("student_id") ?? "";
  if (!/^\d+$/.test(studentId)) {
    return NextResponse.json(
      { status: "error", error: "student_id is required" },
      { status: 400 },
    );
  }

  const session = await parentAuth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const backendUrl = `${getServerApiUrl()}/parent/me/news/${encodeURIComponent(announcementId)}/declaration/proof?student_id=${encodeURIComponent(studentId)}&format=pdf`;

  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: { Authorization: `Bearer ${bearer}` },
      cache: "no-store",
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedParentAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("parent_declaration_proof_pdf_proxy_non_ok", {
      announcement_id: announcementId,
      status: upstream.status,
    });
    return forwardBackendResponse(upstream);
  }

  const headers = new Headers();
  for (const name of [
    "content-type",
    "content-disposition",
    "cache-control",
    "content-length",
    "x-content-type-options",
  ]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }

  return new NextResponse(upstream.body, { status: 200, headers });
}

export const GET = withParentAuth(GETHandler);
