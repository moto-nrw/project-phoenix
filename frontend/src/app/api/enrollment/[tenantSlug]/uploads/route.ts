// Anonymous proxy for a proof file (Nachweis) a parent attaches to an
// enrollment (#4783). Like the submit proxy it carries no session and
// forwards only the canonical client IP, which the backend's public limiter
// buckets on. The body is bounded here before it is buffered; the backend
// checks size, kind by content and the field's own rules again.

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

const logger = createLogger({ component: "EnrollmentUploadProxy" });

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ tenantSlug?: string }> },
) {
  const { tenantSlug } = await context.params;
  if (typeof tenantSlug !== "string" || !tenantSlug) {
    return problemResponse(400, "tenant slug is required", request);
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

  // Forward only the parts the backend reads.
  const file = incoming.get("file");
  const outgoing = new FormData();
  for (const name of [
    "phase_id",
    "field_key",
    "late_invite_token",
    "status_token",
  ]) {
    const value = incoming.get(name);
    if (typeof value === "string") outgoing.append(name, value);
  }
  if (file instanceof File) outgoing.append("file", file, file.name);

  const forwardedFor = canonicalForwardedFor(request.headers);
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/${encodeURIComponent(tenantSlug)}/uploads`,
      {
        method: "POST",
        headers: forwardedFor ? { "X-Forwarded-For": forwardedFor } : {},
        body: outgoing,
      },
    );
    return forwardBackendResponse(response, request);
  } catch (error) {
    captureBffException(error, request);
    logger.error("enrollment_upload_proxy_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}
