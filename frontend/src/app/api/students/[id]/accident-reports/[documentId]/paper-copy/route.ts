import { backendResponseError } from "~/lib/api-response-error.server";
// Accident reports (#4771): upload proxy for the signed paper copy. Like the
// child document upload, this route mirrors the size/MIME gate and forwards
// the multipart body unchanged; the backend checks the content, the
// student_accidents:manage permission and that the report is still open.

import type { NextRequest } from "next/server";
import {
  createFileUploadHandler,
  FileValidationError,
} from "~/lib/file-upload-wrapper.server";
import { uncachedAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";

interface BackendDocumentResponse {
  data?: unknown;
}

export const POST = createFileUploadHandler<unknown>(
  async (request: NextRequest, formData: FormData, token: string) => {
    const { studentId, documentId } = extractIdsFromUrl(request);

    const file = formData.get("file");
    if (!file || !(file instanceof File)) {
      throw new FileValidationError(
        "No document file provided",
        400,
        "files.file_missing",
      );
    }

    const backendUrl = `${getServerApiUrl()}/api/students/${studentId}/accident-reports/${documentId}/paper-copy`;
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

    if (!response.ok) throw await backendResponseError(response);

    const body = (await response.json()) as BackendDocumentResponse;
    return body.data ?? {};
  },
  {
    maxSizeInMB: 10,
    allowedMimeTypes: [
      "",
      "application/octet-stream",
      "application/pdf",
      "image/png",
      "image/jpeg",
      "image/jpg",
    ],
    allowedExtensions: [".pdf", ".png", ".jpg", ".jpeg"],
  },
);

// Pulls both ids out of the URL because createFileUploadHandler doesn't
// expose `params` to the body callback. Throws so a regex drift can never
// target the wrong child or report.
function extractIdsFromUrl(request: NextRequest): {
  studentId: string;
  documentId: string;
} {
  const match = request.nextUrl.pathname.match(
    /\/students\/(\d+)\/accident-reports\/(\d+)\/paper-copy/,
  );
  const studentId = match?.[1];
  const documentId = match?.[2];
  if (!studentId || !documentId) {
    throw new Error("Could not extract student or report ID from request URL");
  }
  return { studentId, documentId };
}
