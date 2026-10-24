import { ExternalLink } from "lucide-react";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import type { AdminRequestUpload } from "~/lib/enrollment-admin-api";
import {
  adminUploadHref,
  asUploadValues,
  formatUploadSize,
} from "~/lib/enrollment-uploads";

/**
 * The proof files (Nachweise) of one file question in the staff view of a
 * request (#4783). Each file opens in a new tab through the download proxy
 * when the reader holds „Nachweise aus Anmeldungen ansehen“; without it the
 * name stays visible and says why it cannot be opened. A file removed after
 * the retention period says so instead of offering a dead link.
 */
export function EnrollmentRequestUploadList({
  requestId,
  value,
  uploads,
  canView,
}: Readonly<{
  requestId: string;
  value: unknown;
  /** Undefined when the endpoint does not report stored files. */
  uploads?: readonly AdminRequestUpload[];
  canView?: boolean;
}>) {
  const files = asUploadValues(value);
  if (files.length === 0) return null;
  const stored = new Map((uploads ?? []).map((upload) => [upload.id, upload]));

  return (
    <ul className="space-y-1.5 text-sm">
      {files.map((file) => {
        const uploadId = "upload_id" in file ? file.upload_id : null;
        const upload = uploadId ? stored.get(uploadId) : undefined;
        const deleted = uploads !== undefined && !upload;
        const key = uploadId ?? ("token" in file ? file.token : file.filename);
        return (
          <li key={key} className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <FileTypeIcon
              contentType={upload?.content_type ?? file.content_type}
              className="h-4 w-4 shrink-0"
            />
            <span className="min-w-0 font-medium break-all text-gray-900">
              {upload?.filename ?? (file.filename || "Datei")}
            </span>
            {deleted ? (
              <span className="text-gray-500">
                Nachweis gelöscht (Löschfrist abgelaufen)
              </span>
            ) : (
              <>
                <span className="text-gray-500">
                  {formatUploadSize(upload?.size_bytes ?? file.size_bytes)}
                </span>
                {canView === true && uploadId ? (
                  <a
                    href={adminUploadHref(requestId, uploadId)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 font-medium text-gray-900 underline underline-offset-2 hover:text-gray-600"
                  >
                    Öffnen
                    <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                    <span className="sr-only">
                      {upload?.filename ?? file.filename} in neuem Tab
                    </span>
                  </a>
                ) : null}
                {canView === false ? (
                  <span className="text-gray-500">
                    Keine Berechtigung zum Öffnen
                  </span>
                ) : null}
              </>
            )}
          </li>
        );
      })}
    </ul>
  );
}
