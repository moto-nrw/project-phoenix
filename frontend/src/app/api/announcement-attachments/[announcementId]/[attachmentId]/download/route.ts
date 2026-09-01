// Anhänge an Elternmitteilungen (#2890): Personal-Vorschau der angehängten
// Datei. Spiegelt den Download-Proxy der Dateiablage: JWT serverseitig
// setzen, bei 401 einmal erneuern, den Body mit Content-Type und
// Content-Disposition unverändert durchreichen.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AnnouncementAttachmentDownload" });

async function GETHandler(
  request: NextRequest,
  {
    params,
  }: { params: Promise<{ announcementId: string; attachmentId: string }> },
) {
  const { announcementId, attachmentId } = await params;

  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  // ?inline=1 asks for in-browser viewing (PDF, images); the backend decides
  // per content type and answers with a sandboxing CSP.
  const inline = request.nextUrl.searchParams.get("inline") === "1";
  const backendUrl = `${getServerApiUrl()}/api/announcement-attachments/${encodeURIComponent(announcementId)}/${encodeURIComponent(attachmentId)}/download${inline ? "?inline=1" : ""}`;

  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: { Authorization: `Bearer ${bearer}` },
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("announcement_attachment_download_proxy_non_ok", {
      announcement_id: announcementId,
      attachment_id: attachmentId,
      status: upstream.status,
    });
    return new NextResponse(null, { status: upstream.status });
  }

  const headers = new Headers();
  // nosniff is the one that matters here: an uploaded file must never be
  // re-interpreted as something executable because a Content-Type looked
  // wrong.
  for (const name of [
    "content-type",
    "content-disposition",
    "cache-control",
    "content-length",
    "x-content-type-options",
    "content-security-policy",
  ]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }

  return new NextResponse(upstream.body, { status: 200, headers });
}

export const GET = withTenantAuth(GETHandler);
