import type { NextRequest } from "next/server";
import {
  createFileUploadHandler,
  FileValidationError,
} from "~/lib/file-upload-wrapper.server";
import { backendResponseError } from "~/lib/api-helpers.server";

interface EnrollmentLegalDocumentResponse {
  document_url: string;
}

export const POST = createFileUploadHandler<EnrollmentLegalDocumentResponse>(
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
    const url = `${getServerApiUrl()}/api/enrollment/legal-documents`;
    const response = await fetch(url, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: validatedFormData,
    });

    if (!response.ok) {
      throw await backendResponseError(response);
    }

    return (
      (await response.json()) as { data: EnrollmentLegalDocumentResponse }
    ).data;
  },
  {
    maxSizeInMB: 10,
    allowedMimeTypes: ["application/pdf"],
    allowedExtensions: [".pdf"],
  },
);
