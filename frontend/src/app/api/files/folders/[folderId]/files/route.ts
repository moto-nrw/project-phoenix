// Dateiablage (#2596): file list + multipart upload proxy. The backend owns
// the authoritative magic-number validation and the visibility / upload
// permission checks; this route mirrors the size and type gate so oversized
// or obviously wrong files never leave the Next.js server, and forwards the
// multipart body unchanged. On 401 we refresh the access token via
// uncachedAuth() and retry once, mirroring the child document upload proxy.

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

const logger = createLogger({ component: "FilesUploadRoute" });

export const GET = proxyGet(
  (p) => `/api/files/folders/${requirePathSegmentParam(p, "folderId")}/files`,
);

interface BackendFileResponse {
  status?: string;
  message?: string;
  data?: unknown;
}

export const POST = createFileUploadHandler<unknown>(
  async (request: NextRequest, formData: FormData, token: string) => {
    const folderId = extractFolderIdFromUrl(request);

    const file = formData.get("file");
    if (!file || !(file instanceof File)) {
      throw new FileValidationError("No file provided", 400);
    }

    const backendUrl = `${getServerApiUrl()}/api/files/folders/${folderId}/files`;
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
      logger.error("file_upload_proxy_failed", {
        folder_id: folderId,
        status: response.status,
        error: errorText,
      });
      // The "API error (XXX)" prefix keeps the backend's status code intact
      // through handleApiError instead of collapsing everything into a 500.
      throw new Error(
        `API error (${response.status}): ${errorText || "Upload fehlgeschlagen"}`,
      );
    }

    const body = (await response.json()) as BackendFileResponse;
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

// Pulls {folderId} out of the URL because createFileUploadHandler doesn't
// expose `params` to the body callback. Throws so a regex drift can never
// silently target the wrong folder.
function extractFolderIdFromUrl(request: NextRequest): string {
  const match = request.nextUrl.pathname.match(
    /\/files\/folders\/([^/]+)\/files/,
  );
  const id = match?.[1];
  if (!id) {
    throw new Error("Could not extract folder ID from request URL");
  }
  return id;
}
