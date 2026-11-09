import {
  DOCUMENT_UPLOAD,
  proxyFileUpload,
} from "~/lib/file-upload-wrapper.server";
import { proxyGet } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Child documents (#777). The backend checks the content and the permission of
// the category; the size and type gate here keeps obviously wrong files on
// this server.
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/documents`,
);

export const POST = proxyFileUpload(
  (p) => `/api/students/${requirePathSegmentParam(p)}/documents`,
  { ...DOCUMENT_UPLOAD, file: "file", fields: ["category"] },
);
