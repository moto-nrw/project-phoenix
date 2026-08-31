"use client";

import { Download, X } from "lucide-react";
import { Button } from "~/components/ui/button";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { formatBytes, isViewableInBrowser } from "~/lib/files-api";

/**
 * One attached file, in the shape both the staff and the parents side use.
 */
export interface AttachmentListItem {
  id: string;
  filename: string;
  size_bytes: number;
  content_type: string;
}

interface AttachmentListProps {
  attachments: readonly AttachmentListItem[];
  /** Builds the download address for one attachment. */
  downloadUrl: (attachmentId: string) => string;
  /** When set, each row gets a remove button calling this. */
  onRemove?: (attachmentId: string) => void;
  /** Disables the remove buttons while a request is in flight. */
  busy?: boolean;
  removeLabel?: string;
  downloadLabel?: string;
  openLabel?: string;
}

/**
 * The list of files attached to something, with a download link per row.
 *
 * Shared between the Elternmitteilungs-Assistent and the parents portal
 * (#2890) so a parent sees the same file, named and sized the same way, that
 * the person who attached it saw.
 *
 * The filename is the link, and the row carries no other click target: a row
 * that looks pressable but only reacts on one word is exactly the kind of
 * read-wrong this codebase keeps paying for.
 */
export function AttachmentList({
  attachments,
  downloadUrl,
  onRemove,
  busy = false,
  removeLabel = "Anhang entfernen",
  downloadLabel = "Herunterladen",
  openLabel = "Im Browser öffnen",
}: AttachmentListProps) {
  if (attachments.length === 0) return null;

  return (
    <ul className="flex flex-col gap-2">
      {attachments.map((attachment) => {
        const viewable = isViewableInBrowser(attachment.content_type);
        const href = viewable
          ? `${downloadUrl(attachment.id)}?inline=1`
          : downloadUrl(attachment.id);
        return (
          <li
            key={attachment.id}
            className="flex items-center gap-3 rounded-md border border-gray-200 bg-white px-3 py-2"
          >
            <FileTypeIcon contentType={attachment.content_type} />
            <a
              href={href}
              title={viewable ? openLabel : downloadLabel}
              {...(viewable ? { target: "_blank", rel: "noopener" } : {})}
              className="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 hover:underline"
            >
              {attachment.filename}
            </a>
            <span className="shrink-0 text-xs text-gray-500">
              {formatBytes(attachment.size_bytes)}
            </span>
            {onRemove ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                disabled={busy}
                aria-label={`${removeLabel}: ${attachment.filename}`}
                title={removeLabel}
                onClick={() => onRemove(attachment.id)}
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </Button>
            ) : (
              <Download
                className="h-4 w-4 shrink-0 text-gray-400"
                aria-hidden="true"
              />
            )}
          </li>
        );
      })}
    </ul>
  );
}
