import { apiDelete } from "~/lib/api-helpers.server";
import { unwrapEnvelope } from "~/lib/backend-client.server";
import {
  FileValidationError,
  proxyFileUpload,
} from "~/lib/file-upload-wrapper.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";
import type { BackendProfile } from "~/lib/profile-helpers";

/** JPEG, PNG and GIF by their first bytes; WebP as RIFF with "WEBP" at 8. */
function isImage(bytes: Uint8Array): boolean {
  const hex = Array.from(bytes.subarray(0, 4))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
  if (hex.startsWith("ffd8ff") || hex === "89504e47" || hex === "47494638") {
    return true;
  }
  return (
    hex === "52494646" &&
    new TextDecoder().decode(bytes.subarray(8, 12)) === "WEBP"
  );
}

export const POST = proxyFileUpload("/api/me/profile/avatar", {
  file: "avatar",
  maxSizeInMB: 5,
  allowedMimeTypes: [
    "image/jpeg",
    "image/jpg",
    "image/png",
    "image/gif",
    "image/webp",
  ],
  allowedExtensions: [".jpg", ".jpeg", ".png", ".gif", ".webp"],
  verify: (buffer) => {
    if (!isImage(new Uint8Array(buffer))) {
      throw new FileValidationError(
        "file content is not an image",
        415,
        "files.file_type_not_allowed",
      );
    }
  },
});

export const DELETE = createDeleteHandler(async (_request, token) =>
  unwrapEnvelope<BackendProfile>(
    await apiDelete("/api/me/profile/avatar", token),
  ),
);
