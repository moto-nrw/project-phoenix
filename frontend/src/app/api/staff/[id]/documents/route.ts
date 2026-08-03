// Staff documents (#1424): list + multipart upload proxy. The backend owns
// the authoritative magic-number validation and the per-category permission
// checks; this route mirrors the size/MIME gate so oversized or obviously
// wrong files never leave the Next.js server, and forwards the multipart
// body unchanged. On 401 we refresh the access token via uncachedAuth() and
// retry once, mirroring the student-photo upload proxy.

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

const logger = createLogger({ component: "StaffDocumentsRoute" });

export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/documents`,
);

interface BackendDocumentResponse {
  status?: string;
  message?: string;
  data?: unknown;
}

export const POST = createFileUploadHandler<unknown>(
  async (request: NextRequest, formData: FormData, token: string) => {
    const staffId = extractStaffIdFromUrl(request);

    const file = formData.get("file");
    if (!file || !(file instanceof File)) {
      throw new FileValidationError("No document file provided", 400);
    }
    const category = formData.get("category");
    if (typeof category !== "string" || category === "") {
      throw new FileValidationError("No document category provided", 400);
    }

    const backendUrl = `${getServerApiUrl()}/api/staff/${staffId}/documents`;
    // Rebuild the form from the in-memory file so the body is re-readable
    // for the post-refresh retry.
    const buffer = await file.arrayBuffer();
    const forwarded = new FormData();
    forwarded.append(
      "file",
      new File([buffer], file.name, { type: file.type }),
    );
    forwarded.append("category", category);

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
      logger.error("staff_document_upload_proxy_failed", {
        staff_id: staffId,
        status: response.status,
        error: errorText,
      });
      // The "API error (XXX)" prefix keeps the backend's status code (400
      // invalid file, 403 category permission, 404 staff) intact through
      // handleApiError instead of collapsing everything into a 500.
      throw new Error(
        `API error (${response.status}): ${errorText || "Upload fehlgeschlagen"}`,
      );
    }

    const body = (await response.json()) as BackendDocumentResponse;
    return body.data ?? {};
  },
  {
    maxSizeInMB: 10,
    // DOCX is an OOXML ZIP container, so browsers without the OOXML mapping
    // label a valid .docx as a plain ZIP (or not at all). Accepting the ZIP
    // labels here costs nothing: the extension gate below still requires
    // .docx, and the backend rejects an archive that lacks the OOXML parts
    // (api/common/upload_documents.go).
    allowedMimeTypes: [
      "",
      "application/octet-stream",
      "application/pdf",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/zip",
      "application/x-zip-compressed",
      "image/png",
      "image/jpeg",
      "image/jpg",
    ],
    allowedExtensions: [".pdf", ".docx", ".png", ".jpg", ".jpeg"],
  },
);

// Pulls {id} out of the URL because createFileUploadHandler doesn't expose
// `params` to the body callback. Throws so a regex drift can never silently
// target the wrong staff member.
function extractStaffIdFromUrl(request: NextRequest): string {
  const match = request.nextUrl.pathname.match(/\/staff\/([^/]+)\/documents/);
  const id = match?.[1];
  if (!id) {
    throw new Error("Could not extract staff ID from request URL");
  }
  return id;
}
