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
import {
  parentAuth,
  uncachedParentAuth,
  withParentAuth,
} from "~/server/auth/parent";

const logger = createLogger({ component: "ParentEnrollmentUploadProxy" });

// Parent-authenticated proxy for proof files. It carries the parent portal's
// session to the matching backend route, where relationship-level audience
// access is derived. Browser form fields never decide that access.
export const POST = withParentAuth(
  async (
    request: NextRequest,
    context: { params: Promise<{ tenantSlug?: string }> },
  ) => {
    const session = await parentAuth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }
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

    const outgoing = new FormData();
    for (const name of ["phase_id", "field_key", "late_invite_token"]) {
      const value = incoming.get(name);
      if (typeof value === "string") outgoing.append(name, value);
    }
    const file = incoming.get("file");
    if (file instanceof File) outgoing.append("file", file, file.name);

    const forwardedFor = canonicalForwardedFor(request.headers);
    const sendUpload = (token: string) =>
      fetch(
        `${getServerApiUrl()}/api/parent/enrollments/${encodeURIComponent(tenantSlug)}/uploads`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
            ...(forwardedFor ? { "X-Forwarded-For": forwardedFor } : {}),
          },
          body: outgoing,
        },
      );
    try {
      let response = await sendUpload(session.user.token);
      if (response.status === 401) {
        const refreshed = await uncachedParentAuth();
        if (
          refreshed?.user?.token &&
          refreshed.user.token !== session.user.token
        ) {
          response = await sendUpload(refreshed.user.token);
        }
      }
      return forwardBackendResponse(response, request);
    } catch (error) {
      captureBffException(error, request);
      logger.error("parent_enrollment_upload_proxy_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return problemResponse(500, "Internal Server Error", request);
    }
  },
);
