import {
  File as FileIcon,
  FileImage,
  FileSpreadsheet,
  FileText,
  Presentation,
} from "lucide-react";

/**
 * Icon for an uploaded file, picked from its content type.
 *
 * Shared because two screens now show the same set of uploads — the school
 * file storage (#2596) and the attachments of an Elternmitteilung (#2890) —
 * and a file that looks like a spreadsheet in one place must not look like a
 * blank sheet in the other.
 */
export function FileTypeIcon({
  contentType,
  className = "h-4 w-4 text-gray-400",
}: {
  contentType: string;
  className?: string;
}) {
  if (contentType.startsWith("image/")) {
    return <FileImage className={className} aria-hidden="true" />;
  }
  if (contentType === "application/pdf") {
    return <FileText className={className} aria-hidden="true" />;
  }
  if (contentType.includes("spreadsheetml")) {
    return <FileSpreadsheet className={className} aria-hidden="true" />;
  }
  if (contentType.includes("presentationml")) {
    return <Presentation className={className} aria-hidden="true" />;
  }
  return <FileIcon className={className} aria-hidden="true" />;
}
