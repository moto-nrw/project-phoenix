// Anhänge einer Elternmitteilung (#2890), Elternseite: die Datei selbst.
//
// Eigener Handler statt des JSON-Proxys, weil hier Bytes durchlaufen. Der
// Empfängerkreis der Mitteilung entscheidet im Backend; wer nicht dazugehört,
// bekommt 404 — nicht 403, sonst ließe sich an der Antwort ablesen, dass es
// die Mitteilung gibt.

import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { parentAuth, uncachedParentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentAttachmentDownload" });

async function GETHandler(
  request: NextRequest,
  {
    params,
  }: { params: Promise<{ announcementId: string; attachmentId: string }> },
) {
  const { announcementId, attachmentId } = await params;

  const session = await parentAuth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const inline = request.nextUrl.searchParams.get("inline") === "1";
  const backendUrl = `${getServerApiUrl()}/parent-news-attachments/${encodeURIComponent(announcementId)}/${encodeURIComponent(attachmentId)}/download${inline ? "?inline=1" : ""}`;

  const makeRequest = (bearer: string) =>
    fetch(backendUrl, {
      method: "GET",
      headers: { Authorization: `Bearer ${bearer}` },
    });

  let upstream = await makeRequest(token);
  if (upstream.status === 401) {
    const refreshed = await uncachedParentAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      upstream = await makeRequest(refreshed.user.token);
    }
  }

  if (!upstream.ok) {
    logger.warn("parent_attachment_download_proxy_non_ok", {
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

export const GET = withParentAuth(GETHandler);
