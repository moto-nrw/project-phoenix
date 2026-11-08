import { proxyFileUpload } from "~/lib/file-upload-wrapper.server";
import { proxyGet } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) => `/api/files/folders/${requirePathSegmentParam(p, "folderId")}/files`,
);

export const POST = proxyFileUpload(
  (p) => `/api/files/folders/${requirePathSegmentParam(p, "folderId")}/files`,
  {
    file: "file",
    maxSizeInMB: 25,
    // OOXML containers are ZIP archives; browsers without the OOXML mapping
    // label them as plain ZIP (or not at all). The extension gate below still
    // requires the office extension, and the backend rejects an archive that
    // lacks the OOXML parts (api/common/upload_documents.go).
    allowedMimeTypes: [
      "",
      "application/octet-stream",
      "application/pdf",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      "application/zip",
      "application/x-zip-compressed",
      "image/png",
      "image/jpeg",
      "image/jpg",
    ],
    allowedExtensions: [
      ".pdf",
      ".docx",
      ".xlsx",
      ".pptx",
      ".png",
      ".jpg",
      ".jpeg",
    ],
  },
);
