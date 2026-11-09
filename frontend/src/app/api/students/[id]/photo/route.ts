import {
  FileValidationError,
  proxyFileUpload,
} from "~/lib/file-upload-wrapper.server";
import {
  JPEG_SIGNATURE,
  PNG_SIGNATURE,
  WEBP_SIGNATURE,
  validateImageMagicBytes,
} from "~/lib/image-magic-bytes";
import { proxyDelete } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const photoPath = (p: Record<string, unknown>) =>
  `/api/students/${requirePathSegmentParam(p)}/photo`;

// consent_acknowledged lets the backend stamp photo_consent_given_at with the
// upload when the child has no recorded consent yet. The backend also rejects
// GIFs (AllowedImageTypes), so they stop here.
export const POST = proxyFileUpload(photoPath, {
  file: "photo",
  optionalFields: ["consent_acknowledged"],
  maxSizeInMB: 5,
  allowedMimeTypes: ["image/jpeg", "image/jpg", "image/png", "image/webp"],
  allowedExtensions: [".jpg", ".jpeg", ".png", ".webp"],
  verify: (bytes) => {
    if (
      !validateImageMagicBytes(bytes, [
        JPEG_SIGNATURE,
        PNG_SIGNATURE,
        WEBP_SIGNATURE,
      ])
    ) {
      throw new FileValidationError(
        "Ungültiges Bildformat. Bitte JPEG, PNG oder WebP hochladen.",
        415,
        "files.file_type_not_allowed",
      );
    }
  },
});

export const DELETE = proxyDelete(photoPath);
