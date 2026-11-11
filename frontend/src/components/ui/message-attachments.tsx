"use client";

import { useState } from "react";
import { Download } from "lucide-react";

import { cn } from "~/lib/utils";
import { buttonClassName } from "./button";
import { FileTypeIcon } from "./file-type-icon";
import {
  ImageLightbox,
  type ImageLightboxLabels,
  type LightboxImage,
} from "./image-lightbox";

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
  /** "Herunterladen: {name}" */
  readonly downloadFile: (filename: string) => string;
  /** "+3 weitere Bilder" */
  readonly moreImages: (count: number) => string;
  readonly lightbox?: ImageLightboxLabels;
}

const GERMAN_LABELS: MessageAttachmentLabels = {
  openImage: (filename) => `Bild öffnen: ${filename}`,
  downloadFile: (filename) => `Herunterladen: ${filename}`,
  moreImages: (count) =>
    count === 1 ? "1 weiteres Bild" : `${count} weitere Bilder`,
};

function isImage(contentType: string): boolean {
  return contentType === "image/png" || contentType === "image/jpeg";
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1).replace(".", ",")} MB`;
}

const KIND_LABELS: ReadonlyArray<readonly [string, string]> = [
  ["application/pdf", "PDF"],
  ["wordprocessingml", "Word"],
  ["spreadsheetml", "Excel"],
  ["presentationml", "PowerPoint"],
];

function kindLabel(contentType: string): string | null {
  return (
    KIND_LABELS.find(([needle]) => contentType.includes(needle))?.[1] ?? null
  );
}

/** At most four tiles; the fourth names the pictures it stands for. */
const MAX_TILES = 4;

/** A single picture keeps its own shape between portrait 3:4 and wide 16:9. */
function singleAspect(item: MessageAttachmentItem): number {
  if (!item.width || !item.height) return 4 / 3;
  return Math.min(16 / 9, Math.max(3 / 4, item.width / item.height));
}

/**
 * The photos and files of one chat message (#6685): pictures as a tight
 * grid that opens the viewer, every other file as a row that downloads it.
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

  const images = attachments.filter((item) => isImage(item.content_type));
  const files = attachments.filter((item) => !isImage(item.content_type));
  const lightboxImages: LightboxImage[] = images.map((item) => ({
    src: `${downloadUrl(item.id)}?inline=1`,
    name: item.filename,
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
      {files.map((item) => {
        const kind = kindLabel(item.content_type);
        return (
          <div
            key={item.id}
            className="flex w-full max-w-xs items-center gap-3 rounded-lg border border-gray-200 bg-white py-1.5 pr-1.5 pl-3 text-left text-gray-900"
          >
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-gray-100">
              <FileTypeIcon
                contentType={item.content_type}
                className="h-4.5 w-4.5 text-gray-600"
              />
            </span>
            <span className="min-w-0 flex-1">
              <a
                href={downloadUrl(item.id)}
                download={item.filename}
                className="block truncate text-sm font-medium outline-none hover:underline focus-visible:underline"
              >
                {item.filename}
              </a>
              <span className="block text-xs text-gray-500">
                {kind ? `${kind} · ` : ""}
                {formatSize(item.size_bytes)}
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
                className: "shrink-0",
              })}
            >
              <Download className="h-4 w-4" aria-hidden="true" />
            </a>
          </div>
        );
      })}
      <ImageLightbox
        images={lightboxImages}
        index={viewing}
        onIndexChange={setViewing}
        onClose={() => setViewing(null)}
        labels={labels.lightbox}
      />
    </div>
  );
}
