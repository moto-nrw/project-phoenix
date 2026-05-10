// Student photo upload + delete proxy. Mirrors /api/me/profile/avatar
// (magic-byte + MIME + size validation). On 401 we refresh the access
// token via uncachedAuth() and retry once.

import type { NextRequest } from "next/server";
import {
  createFileUploadHandler,
  FileValidationError,
} from "~/lib/file-upload-wrapper";
import { createDeleteHandler } from "~/lib/route-wrapper";
import { uncachedAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import {
  JPEG_SIGNATURE,
  PNG_SIGNATURE,
  WEBP_SIGNATURE,
  validateImageMagicBytes,
} from "~/lib/image-magic-bytes";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentPhotoRoute" });

interface BackendPhotoResponse {
  status?: string;
  message?: string;
  data?: { photo_url: string };
}

interface UploadResult {
  photo_url: string;
}

// POST handler — multipart upload, field name "photo" (matches backend
// uploadStudentPhoto). Validation cap matches the backend constant
// `maxStudentPhotoBody` (5 MiB) so callers fail at the proxy boundary
// rather than burning a backend call to learn the body was too big.
export const POST = createFileUploadHandler<UploadResult>(
  async (request: NextRequest, formData: FormData, token: string) => {
    const studentId = extractStudentIdFromUrl(request);

    const photoFile = formData.get("photo");
    if (!photoFile || !(photoFile instanceof File)) {
      // 400 — caller-shaped: form is missing the field. FileValidationError
      // travels through the wrapper's catch block and is rendered as a 4xx
      // instead of being collapsed into a 500 by handleApiError.
      throw new FileValidationError("No photo file provided", 400);
    }

    // Magic-byte check via the shared helper. The user-avatar route
    // (api/me/profile/avatar) still has its own copy — migrating that
    // one is out of scope for this branch (it ships GIF support and a
    // declared/detected MIME diff check that we don't need here). Once
    // both routes consume the same helper the two paths can no longer
    // drift on signature semantics.
    const buffer = await photoFile.arrayBuffer();
    const isValidHeader = validateImageMagicBytes(buffer, [
      JPEG_SIGNATURE,
      PNG_SIGNATURE,
      WEBP_SIGNATURE,
    ]);
    if (!isValidHeader) {
      // 415 Unsupported Media Type — the file's MIME header lied. Throwing
      // FileValidationError (rather than a plain Error) keeps the status
      // code intact through the wrapper instead of getting rewritten to
      // 500 by handleApiError, which only extracts status codes from
      // strings prefixed with "API error (XXX):".
      throw new FileValidationError(
        "Ungültiges Bildformat. Bitte JPEG, PNG oder WebP hochladen.",
        415,
      );
    }

    // Re-wrap the buffer into a fresh File since we consumed the stream
    const validatedFile = new File([buffer], photoFile.name, {
      type: photoFile.type,
    });
    const validatedFormData = new FormData();
    validatedFormData.append("photo", validatedFile);
    // Forward the optional consent acknowledgement so the backend can stamp
    // photo_consent_given_at atomically when the row hasn't recorded consent
    // yet. Frontend sends "true" only when the form-level checkbox is on.
    const consentField = formData.get("consent_acknowledged");
    if (typeof consentField === "string" && consentField === "true") {
      validatedFormData.append("consent_acknowledged", "true");
    }

    const backendUrl = `${getServerApiUrl()}/api/students/${studentId}/photo`;
    // FormData backed by an in-memory ArrayBuffer (validatedFile above)
    // is re-readable, so the same body can drive the original request and
    // the post-refresh retry without rebuilding it.
    const sendUpload = (bearer: string) =>
      fetch(backendUrl, {
        method: "POST",
        headers: { Authorization: `Bearer ${bearer}` },
        body: validatedFormData,
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
      logger.error("student_photo_upload_proxy_failed", {
        student_id: studentId,
        status: response.status,
        error: errorText,
      });
      // Format the message so handleApiError can extract the status code
      // — anything else gets collapsed into a 500. This matters in
      // practice for the 4xx cases the backend already speaks: 400
      // missing consent, 403 photos feature disabled, 409 consent-
      // withdrawal race during upload. Without this format every one of
      // those would surface as an internal-server error in the proxy.
      throw new Error(
        `API error (${response.status}): ${errorText || "Upload fehlgeschlagen"}`,
      );
    }

    const body = (await response.json()) as BackendPhotoResponse;
    const photoUrl = body.data?.photo_url;
    if (!photoUrl) {
      // Backend should never reach this; treat as a real 500 since the
      // contract is broken.
      throw new Error("Backend hat keine photo_url zurückgegeben");
    }
    return { photo_url: photoUrl };
  },
  {
    maxSizeInMB: 5,
    // Tighter than the avatar route — no GIFs for student photos. The
    // backend's AllowedImageTypes also rejects GIFs, so accepting them
    // here would only push the rejection one hop further.
    allowedMimeTypes: ["image/jpeg", "image/jpg", "image/png", "image/webp"],
    allowedExtensions: [".jpg", ".jpeg", ".png", ".webp"],
  },
);

// DELETE handler — idempotent, backend returns 200 even when no photo set.
//
// 401 retry mirrors the POST handler above. createDeleteHandler hands us a
// single cached token; when it has expired but the NextAuth refresh token is
// still valid the backend returns 401, and without the inline retry the user
// sees a hard "Foto konnte nicht entfernt werden" error until they reload
// the page. The generic createDeleteHandler retry path only triggers on
// errors whose message includes the literal "API error (401)" (see
// route-wrapper.ts is401Error), and we throw the backend body / a German
// fallback instead — so the wrapper-level retry never fires here. We do the
// refresh ourselves rather than reshaping the thrown message because the
// upload path next door already owns this pattern; doing the same thing the
// same way keeps both photo-mutation entry points symmetric.
export const DELETE = createDeleteHandler(
  async (request: NextRequest, token: string) => {
    const studentId = extractStudentIdFromUrl(request);

    const backendUrl = `${getServerApiUrl()}/api/students/${studentId}/photo`;
    const sendDelete = (bearer: string) =>
      fetch(backendUrl, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${bearer}` },
      });

    let response = await sendDelete(token);
    if (response.status === 401) {
      const refreshed = await uncachedAuth();
      if (refreshed?.user?.token && refreshed.user.token !== token) {
        response = await sendDelete(refreshed.user.token);
      }
    }

    if (!response.ok) {
      const errorText = await response.text();
      // Match the upload path's error envelope so handleApiError preserves
      // the status code instead of collapsing 4xx into 500. Same rationale
      // applies on delete: a 403 (feature disabled) or 404 (no photo) must
      // not surface as an internal error.
      throw new Error(
        `API error (${response.status}): ${errorText || "Foto konnte nicht entfernt werden"}`,
      );
    }
    return { success: true };
  },
);

// Pulls {id} out of the URL because createFileUploadHandler doesn't expose
// `params` to the body callback today. Falls back to throwing so a regex
// drift here can never silently target the wrong student.
function extractStudentIdFromUrl(request: NextRequest): string {
  const match = request.nextUrl.pathname.match(/\/students\/([^/]+)\/photo/);
  const id = match?.[1];
  if (!id) {
    throw new Error("Could not extract student ID from request URL");
  }
  return id;
}
