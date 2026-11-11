"use client";

import { useState } from "react";
import { Download } from "lucide-react";

import { fetchMessageAttachmentFile } from "~/lib/message-attachments";
import { cn } from "~/lib/utils";
import {
  AttachmentViewer,
  fileKindLabel,
  formatAttachmentSize,
  isViewerImage,
  type AttachmentViewerLabels,
  type ViewerItem,
} from "./attachment-viewer";
import { buttonClassName } from "./button";
import { FileTypeIcon } from "./file-type-icon";

/** One file of a message, in the shape every chat delivers. */
interface MessageAttachmentItem {
  readonly id: string;
  readonly filename: string;
  readonly size_bytes: number;
  readonly content_type: string;
  readonly width?: number;
  readonly height?: number;
}

export interface MessageAttachmentLabels {
  /** "Bild öffnen: {name}" */
  readonly openImage: (filename: string) => string;
  /** "Datei öffnen: {name}" */
  readonly openFile: (filename: string) => string;
  /** "Herunterladen: {name}" */
  readonly downloadFile: (filename: string) => string;
  /** "+3 weitere Bilder" */
  readonly moreImages: (count: number) => string;
  readonly lightbox?: AttachmentViewerLabels;
}

const GERMAN_LABELS: MessageAttachmentLabels = {
  openImage: (filename) => `Bild öffnen: ${filename}`,
  openFile: (filename) => `Datei öffnen: ${filename}`,
  downloadFile: (filename) => `Herunterladen: ${filename}`,
  moreImages: (count) =>
    count === 1 ? "1 weiteres Bild" : `${count} weitere Bilder`,
};

/** At most four tiles; the fourth names the pictures it stands for. */
const MAX_TILES = 4;

/** A single picture keeps its own shape between portrait 3:4 and wide 16:9. */
function singleAspect(item: MessageAttachmentItem): number {
  if (!item.width || !item.height) return 4 / 3;
  return Math.min(16 / 9, Math.max(3 / 4, item.width / item.height));
}

/**
 * The photos and files of one chat message (#6685): pictures as a tight
 * grid, every other file as a row; a tap on either opens the viewer, the
 * row's arrow saves the file at once.
 * Both portals and all three chats render a message's files through this,
 * so a photo looks the same to the person who sent it and the one who got it.
 */
export function MessageAttachments({
  attachments,
  downloadUrl,
  align = "start",
  labels = GERMAN_LABELS,
}: Readonly<{
  attachments: readonly MessageAttachmentItem[];
  /** Builds the address that saves one attachment. */
  downloadUrl: (attachmentId: string) => string;
  /** Which edge the block hugs: the sender's own messages sit at the end. */
  align?: "start" | "end";
  labels?: MessageAttachmentLabels;
}>) {
  const [viewing, setViewing] = useState<number | null>(null);
  if (attachments.length === 0) return null;

  const images = attachments.filter((item) => isViewerImage(item.content_type));
  const files = attachments.filter((item) => !isViewerImage(item.content_type));
  // The viewer pages in the order the message shows them: pictures, files.
  const viewerItems: ViewerItem[] = [...images, ...files].map((item) => ({
    src: `${downloadUrl(item.id)}?inline=1`,
    name: item.filename,
    contentType: item.content_type,
    sizeBytes: item.size_bytes,
    downloadHref: downloadUrl(item.id),
  }));
  const tiles = images.slice(0, MAX_TILES);
  const hidden = images.length - tiles.length;
  const single = images.length === 1 ? images[0] : undefined;

  return (
    <div
      className={cn(
        "flex w-full flex-col gap-1.5",
        align === "end" ? "items-end" : "items-start",
      )}
    >
      {tiles.length > 0 ? (
        <div
          className={cn(
            "grid gap-0.5 overflow-hidden rounded-xl",
            single ? "w-full max-w-xs grid-cols-1" : "w-full max-w-sm",
            tiles.length === 2 && "grid-cols-2",
            tiles.length >= 3 && "grid-cols-2",
          )}
        >
          {tiles.map((item, position) => {
            const last = position === tiles.length - 1;
            const wide = tiles.length === 3 && position === 0;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setViewing(position)}
                aria-label={labels.openImage(item.filename)}
                className={cn(
                  "group relative block overflow-hidden outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-inset",
                  wide && "col-span-2",
                )}
                style={{
                  aspectRatio: single ? singleAspect(single) : wide ? 2 : 1,
                }}
              >
                {/* eslint-disable-next-line @next/next/no-img-element -- private, session-bound file; next/image cannot fetch it */}
                <img
                  src={`${downloadUrl(item.id)}?inline=1`}
                  alt={item.filename}
                  loading="lazy"
                  decoding="async"
                  width={item.width}
                  height={item.height}
                  className="h-full w-full bg-gray-100 object-cover transition-opacity group-hover:opacity-90"
                />
                {last && hidden > 0 ? (
                  <span className="absolute inset-0 flex items-center justify-center bg-black/55 text-lg font-semibold text-white">
                    <span aria-hidden="true">+{hidden}</span>
                    <span className="sr-only">{labels.moreImages(hidden)}</span>
                  </span>
                ) : null}
              </button>
            );
          })}
        </div>
      ) : null}
      {files.map((item, position) => {
        const kind = fileKindLabel(item.content_type);
        return (
          <div
            key={item.id}
            className="relative flex w-full max-w-xs items-center gap-3 rounded-lg border border-gray-200 bg-white py-1.5 pr-1.5 pl-3 text-left text-gray-900 transition-colors hover:bg-gray-50"
          >
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-gray-100">
              <FileTypeIcon
                contentType={item.content_type}
                className="h-4.5 w-4.5 text-gray-600"
              />
            </span>
            <span className="min-w-0 flex-1">
              {/* The name opens the viewer; its hit area covers the row, the
                  arrow beside it stays its own target. */}
              <button
                type="button"
                onClick={() => setViewing(images.length + position)}
                aria-label={labels.openFile(item.filename)}
                className="block w-full cursor-pointer truncate text-left text-sm font-medium outline-none after:absolute after:inset-0 after:rounded-lg after:content-[''] hover:underline focus-visible:underline focus-visible:after:ring-2 focus-visible:after:ring-gray-900"
              >
                {item.filename}
              </button>
              <span className="block text-xs text-gray-500">
                {kind ? `${kind} · ` : ""}
                {formatAttachmentSize(item.size_bytes)}
              </span>
            </span>
            <a
              href={downloadUrl(item.id)}
              download={item.filename}
              aria-label={labels.downloadFile(item.filename)}
              data-icon-only=""
              className={buttonClassName({
                variant: "ghost",
                size: "icon",
                className: "relative shrink-0",
              })}
            >
              <Download className="h-4 w-4" aria-hidden="true" />
            </a>
          </div>
        );
      })}
      <AttachmentViewer
        items={viewerItems}
        index={viewing}
        onIndexChange={setViewing}
        onClose={() => setViewing(null)}
        loadFile={fetchMessageAttachmentFile}
        labels={labels.lightbox}
      />
    </div>
  );
}
