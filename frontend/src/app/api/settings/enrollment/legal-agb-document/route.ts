import { proxyFileUpload } from "~/lib/file-upload-wrapper.server";
import { proxyDelete } from "~/lib/route-proxy.server";

const path = "/api/settings/enrollment/legal-agb-document";

export const POST = proxyFileUpload(path, {
  file: "document",
  maxSizeInMB: 10,
  allowedMimeTypes: ["application/pdf"],
  allowedExtensions: [".pdf"],
});

export const DELETE = proxyDelete(path);
