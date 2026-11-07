import { backendResponseError } from "~/lib/api-response-error.server";
import type { NextRequest } from "next/server";
import {
  createFileUploadHandler,
  FileValidationError,
} from "~/lib/file-upload-wrapper.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

interface LegalAGBDocumentResponse {
  document_url: string;
}

export const POST = createFileUploadHandler<LegalAGBDocumentResponse>(
  async (_request: NextRequest, formData: FormData, token: string) => {
    const file = formData.get("document");
    if (!file || !(file instanceof File)) {
      throw new FileValidationError(
        "No PDF document provided",
        400,
        "files.file_missing",
      );
    }

    const validatedFormData = new FormData();
    validatedFormData.append("document", file);

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/api/settings/enrollment/legal-agb-document`;
    const response = await fetch(url, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: validatedFormData,
    });

    if (!response.ok) throw await backendResponseError(response);

    return ((await response.json()) as { data: LegalAGBDocumentResponse }).data;
  },
  {
    maxSizeInMB: 10,
    allowedMimeTypes: ["application/pdf"],
    allowedExtensions: [".pdf"],
  },
);

export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string) => {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/api/settings/enrollment/legal-agb-document`;
    const response = await fetch(url, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) throw await backendResponseError(response);

    return null;
  },
);
