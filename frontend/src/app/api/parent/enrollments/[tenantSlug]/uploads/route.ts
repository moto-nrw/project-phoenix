import { problemResponse } from "~/lib/api-response-error.server";
import { MAX_UPLOAD_MB } from "~/lib/enrollment-uploads";
import {
  fileValidationProblemResponse,
  FileValidationError,
  parseBoundedMultipartFormData,
} from "~/lib/file-upload-wrapper.server";
import { parent } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Parent-authenticated proxy for proof files. It carries the parent portal's
// session to the matching backend route, where relationship-level audience
// access is derived. Browser form fields never decide that access.
export const POST = parent.route(async (request, _token, params) => {
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
  return parent.forwardToBackend(
    request,
    `/parent/enrollments/${requirePathSegmentParam(params, "tenantSlug")}/uploads`,
    { method: "POST", body: outgoing },
  );
});
