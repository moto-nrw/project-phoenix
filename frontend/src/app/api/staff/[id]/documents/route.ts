import { proxyFileUpload } from "~/lib/file-upload-wrapper.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/documents`,
);

export const POST = proxyFileUpload(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/documents`,
  {
    file: "file",
    fields: ["category"],
    maxSizeInMB: 10,
    // DOCX is an OOXML ZIP container, so browsers without the OOXML mapping
    // label a valid .docx as a plain ZIP (or not at all). Accepting the ZIP
    // labels here costs nothing: the extension gate below still requires
    // .docx, and the backend rejects an archive that lacks the OOXML parts
    // (api/common/upload_documents.go).
    allowedMimeTypes: [
      "",
      "application/octet-stream",
      "application/pdf",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/zip",
      "application/x-zip-compressed",
      "image/png",
      "image/jpeg",
      "image/jpg",
    ],
    allowedExtensions: [".pdf", ".docx", ".png", ".jpg", ".jpeg"],
  },
);
