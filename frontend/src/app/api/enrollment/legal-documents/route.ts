import { proxyFileUpload } from "~/lib/file-upload-wrapper.server";

export const POST = proxyFileUpload("/api/enrollment/legal-documents", {
  file: "document",
  maxSizeInMB: 10,
  allowedMimeTypes: ["application/pdf"],
  allowedExtensions: [".pdf"],
});
