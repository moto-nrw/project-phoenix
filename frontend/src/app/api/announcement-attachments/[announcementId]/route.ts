// Anhänge an Elternmitteilungen (#2890): Liste und Upload.
//
// Die maßgebliche Prüfung (Magic Bytes, Entwurfs-Zustand, Obergrenze) macht
// das Backend. Dieser Proxy spiegelt Größe und Dateityp nur, damit eine
// offensichtlich falsche Datei den Next.js-Server gar nicht erst verlässt, und
// reicht den Multipart-Body unverändert weiter.

import type { NextRequest } from "next/server";
import {
  createFileUploadHandler,
  FileValidationError,
} from "~/lib/file-upload-wrapper.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { uncachedAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AnnouncementAttachmentsRoute" });

export const GET = proxyGet(
  (p) =>
    `/api/announcement-attachments/${requirePathSegmentParam(p, "announcementId")}`,
);

interface BackendAttachmentResponse {
  status?: string;
  message?: string;
  data?: unknown;
}

export const POST = createFileUploadHandler<unknown>(
  async (request: NextRequest, formData: FormData, token: string) => {
    const announcementId = extractAnnouncementIdFromUrl(request);

    const file = formData.get("file");
    if (!file || !(file instanceof File)) {
      throw new FileValidationError("Bitte wählen Sie eine Datei aus.", 400);
    }

    const backendUrl = `${getServerApiUrl()}/api/announcement-attachments/${announcementId}`;
    // Rebuild the form from the in-memory file so the body is re-readable for
    // the post-refresh retry.
    const buffer = await file.arrayBuffer();
    const forwarded = new FormData();
    forwarded.append(
      "file",
      new File([buffer], file.name, { type: file.type }),
    );

    const sendUpload = (bearer: string) =>
      fetch(backendUrl, {
        method: "POST",
        headers: { Authorization: `Bearer ${bearer}` },
        body: forwarded,
      });

    let response = await sendUpload(token);
    if (response.status === 401) {
      const refreshed = await uncachedAuth();
      if (refreshed?.user?.token && refreshed.user.token !== token) {
        response = await sendUpload(refreshed.user.token);
      }
    }

    if (!response.ok) {
      const errorText = await response.text();
      logger.error("announcement_attachment_upload_proxy_failed", {
        announcement_id: announcementId,
        status: response.status,
        error: errorText,
      });
      // The "API error (XXX)" prefix keeps the backend's status code intact
      // through handleApiError instead of collapsing everything into a 500.
      throw new Error(
        `API error (${response.status}): ${errorText || "Upload fehlgeschlagen"}`,
      );
    }

    const body = (await response.json()) as BackendAttachmentResponse;
    return body.data ?? {};
  },
  {
    maxSizeInMB: 25,
    // OOXML containers are ZIP archives; browsers without the OOXML mapping
    // label them as plain ZIP (or not at all). The extension gate below still
    // requires the office extension, and the backend rejects an archive that
    // lacks the OOXML parts (api/common/upload_documents.go).
    allowedMimeTypes: [
      "",
      "application/octet-stream",
      "application/pdf",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      "application/zip",
      "application/x-zip-compressed",
      "image/png",
      "image/jpeg",
      "image/jpg",
    ],
    allowedExtensions: [
      ".pdf",
      ".docx",
      ".xlsx",
      ".pptx",
      ".png",
      ".jpg",
      ".jpeg",
    ],
  },
);

// Pulls {announcementId} out of the URL because createFileUploadHandler does
// not expose `params` to the body callback. Throws so a regex drift can never
// silently target the wrong announcement.
function extractAnnouncementIdFromUrl(request: NextRequest): string {
  const match = request.nextUrl.pathname.match(
    /\/announcement-attachments\/([^/]+)/,
  );
  const id = match?.[1];
  if (!id) {
    throw new Error("Could not extract announcement ID from request URL");
  }
  return id;
}
