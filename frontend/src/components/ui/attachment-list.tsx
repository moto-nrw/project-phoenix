"use client";

import { Download, ExternalLink, X } from "lucide-react";
import { ActionHint } from "~/components/ui/action-hint";
import { Button, buttonClassName } from "~/components/ui/button";
import { useKitDefault } from "~/components/ui/kit-locale";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { formatBytes, isViewableInBrowser } from "~/lib/file-format";
import { cn } from "~/lib/utils";

/**
 * One attached file, in the shape both the staff and the parents side use.
 * Deliberately not exported: both callers pass their own domain type, which
 * matches structurally — exporting a third name for the same shape would only
 * invite it to drift.
 */
interface AttachmentListItem {
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
  /** What the remove X does (#6744); defaults to „Entfernt den Anhang sofort.“ */
  removeTooltip?: string;
  downloadLabel?: string;
  openLabel?: string;
  /**
   * Shows „Öffnen“ and „Herunterladen“ as actions at the end of the row, the
   * same way the PDFs of a form show them (#5684).
   */
  fileActions?: boolean;
}

const FILE_ACTION_CLASS = buttonClassName({
  variant: "ghost",
  size: "md",
  className: "gap-2",
});

/**
 * The list of files attached to something, with one link per row: a file the
 * browser can show opens in a new tab, any other file downloads.
 *
 * Shared between the Elternmitteilungs-Assistent and the parents portal
 * (#2890) so a parent sees the same file, named and sized the same way, that
 * the person who attached it saw.
 *
 * The filename is the link, and by default the row carries no other click
 * target: a row that looks pressable but only reacts on one word is exactly
 * the kind of read-wrong this codebase keeps paying for. `fileActions` adds
 * named actions, for a list of read-only files; it replaces the remove button.
 */
export function AttachmentList({
  attachments,
  downloadUrl,
  onRemove,
  busy = false,
  removeLabel,
  removeTooltip,
  downloadLabel = "Herunterladen",
  openLabel = "Im Browser öffnen",
  fileActions = false,
}: AttachmentListProps) {
  const kitDefault = useKitDefault();
  if (attachments.length === 0) return null;

  return (
    <ul className="flex flex-col gap-2">
      {attachments.map((attachment) => {
        const viewable = isViewableInBrowser(attachment.content_type);
        const downloadHref = downloadUrl(attachment.id);
        const href = viewable
          ? `${downloadHref}${downloadHref.includes("?") ? "&" : "?"}inline=1`
          : downloadHref;
        return (
          <li
            key={attachment.id}
            className={
              fileActions
                ? "moto-content-surface flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border px-3 py-2"
                : "flex items-center gap-3 rounded-md border border-gray-200 bg-white px-3 py-2"
            }
          >
            <FileTypeIcon contentType={attachment.content_type} />
            <a
              href={href}
              title={viewable ? openLabel : downloadLabel}
              {...(viewable ? { target: "_blank", rel: "noopener" } : {})}
              className={cn(
                "min-w-0 flex-1 truncate text-sm font-medium text-gray-900 hover:underline",
                fileActions && "basis-40",
              )}
            >
              {attachment.filename}
            </a>
            <span className="shrink-0 text-xs text-gray-500">
              {formatBytes(attachment.size_bytes)}
            </span>
            {fileActions ? (
              <div className="flex flex-wrap gap-2">
                {viewable ? (
                  <ActionHint text="Öffnet die Datei in einem neuen Tab.">
                    <a
                      href={href}
                      target="_blank"
                      rel="noopener"
                      aria-label={`${attachment.filename} öffnen`}
                      className={FILE_ACTION_CLASS}
                    >
                      <ExternalLink className="h-4 w-4" aria-hidden="true" />
                      Öffnen
                    </a>
                  </ActionHint>
                ) : null}
                <ActionHint text="Speichert die Datei auf Ihrem Gerät.">
                  <a
                    href={downloadHref}
                    download={attachment.filename}
                    aria-label={`${attachment.filename} herunterladen`}
                    className={FILE_ACTION_CLASS}
                  >
                    <Download className="h-4 w-4" aria-hidden="true" />
                    Herunterladen
                  </a>
                </ActionHint>
              </div>
            ) : onRemove ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                disabled={busy}
                aria-label={`${removeLabel ?? "Anhang entfernen"}: ${attachment.filename}`}
                tooltip={
                  removeTooltip ?? kitDefault("Entfernt den Anhang sofort.")
                }
                onClick={() => onRemove(attachment.id)}
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </Button>
            ) : viewable ? (
              // A viewable file opens in a new tab, so its row must not
              // promise a download (#5330).
              <ExternalLink
                className="h-4 w-4 shrink-0 text-gray-400"
                aria-hidden="true"
              />
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
