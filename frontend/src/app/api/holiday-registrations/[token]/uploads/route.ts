// Anonymous proxy for a proof of benefits a parent attaches to a public
// holiday registration (#5238), like the uploads of the online registration
// (#4783): no session, only the canonical client IP, which the backend's
// upload limit buckets on. The body is bounded here before it is buffered;
// the backend checks size and kind by content again.

import type { NextRequest } from "next/server";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-helpers.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import {
  FileValidationError,
  fileValidationProblemResponse,
  parseBoundedMultipartFormData,
} from "~/lib/file-upload-wrapper.server";
import { MAX_UPLOAD_MB } from "~/lib/enrollment-uploads";
import { createLogger } from "~/lib/logger";
import { captureBffException } from "~/lib/sentry-bff.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { withPrivateNoStore } from "~/lib/private-no-store";

const logger = createLogger({ component: "HolidayProofUploadProxy" });

async function POSTHandler(
  request: NextRequest,
  context: { params: Promise<{ token?: string }> },
) {
  const { token } = await context.params;
  if (typeof token !== "string" || !token) {
    return problemResponse(400, "token required", request);
  }

  let incoming: FormData;
  try {
    incoming = await parseBoundedMultipartFormData(
      request,
      MAX_UPLOAD_MB * 1024 * 1024,
    );
  } catch (error) {
    if (error instanceof FileValidationError) {
      return fileValidationProblemResponse(error, request);
    }
    return problemResponse(400, "Invalid multipart body", request);
  }

  // Forward only the file; the link names the Träger and the period.
  const file = incoming.get("file");
  const outgoing = new FormData();
  if (file instanceof File) outgoing.append("file", file, file.name);

  const forwardedFor = canonicalForwardedFor(request.headers);
  try {
    const response = await fetch(
      `${getServerApiUrl()}/traeger/holiday-registrations/${encodeURIComponent(token)}/uploads`,
      {
        method: "POST",
        headers: forwardedFor ? { "X-Forwarded-For": forwardedFor } : {},
        body: outgoing,
      },
    );
    return forwardBackendResponse(response, request);
  } catch (error) {
    captureBffException(error, request);
    logger.error("holiday_proof_upload_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}

export const POST = withPrivateNoStore(POSTHandler);
