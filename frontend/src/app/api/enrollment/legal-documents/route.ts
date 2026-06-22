import type { NextRequest } from "next/server";
import {
  createFileUploadHandler,
  FileValidationError,
} from "~/lib/file-upload-wrapper.server";
import { uncachedAuth } from "~/server/auth";

interface EnrollmentLegalDocumentResponse {
  document_url: string;
}

async function fetchWithRetry(
  token: string,
  doFetch: (authToken: string) => Promise<Response>,
): Promise<Response> {
  let response = await doFetch(token);

  if (response.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      response = await doFetch(refreshed.user.token);
    }
  }

  return response;
}

export const POST = createFileUploadHandler<EnrollmentLegalDocumentResponse>(
  async (_request: NextRequest, formData: FormData, token: string) => {
    const file = formData.get("document");
    if (!file || !(file instanceof File)) {
      throw new FileValidationError("No PDF document provided", 400);
    }

    const validatedFormData = new FormData();
    validatedFormData.append("document", file);

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/api/enrollment/legal-documents`;
    const response = await fetchWithRetry(token, (t) =>
      fetch(url, {
        method: "POST",
        headers: { Authorization: `Bearer ${t}` },
        body: validatedFormData,
      }),
    );

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(
        `API error (${response.status}): ${errorText || response.statusText}`,
      );
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
