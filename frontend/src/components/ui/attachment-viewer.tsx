"use client";

import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import {
  ChevronLeft,
  ChevronRight,
  Download,
  ExternalLink,
} from "lucide-react";

import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { useScrollLock } from "~/components/ui/hooks/useScrollLock";
import { buttonClassName, Button } from "./button";
import { CloseIcon } from "./close-icon";
import { FileTypeIcon } from "./file-type-icon";
import { LoadErrorAlert } from "./form-error-alert";
import { IconTile } from "./icon-tile";
import { Z_LAYER } from "./overlay-styles";
import { PdfPreview, useObjectUrl, usePdfViewerEnabled } from "./pdf-preview";

const logger = createLogger({ component: "AttachmentViewer" });

/** One picture or file the viewer shows. */
export interface ViewerItem {
  /**
   * The bytes to show: a same-origin address (a sent attachment with
   * `?inline=1`) or a `blob:` URL of a file still on the device.
   */
  readonly src: string;
  /** File name; shown as the caption and read as the item's name. */
  readonly name: string;
  /** MIME type; decides between picture, PDF and the download panel. */
  readonly contentType: string;
  /** Shown beside the type on the download panel. */
  readonly sizeBytes?: number;
  /** Address that saves the file; `src` when omitted. */
  readonly downloadHref?: string;
}

export interface AttachmentViewerLabels {
  readonly close: string;
  readonly previous: string;
  readonly next: string;
  readonly download: string;
  readonly openInNewTab: string;
  /** Office files and every other type a browser cannot show. */
  readonly notViewable: string;
  /** A PDF on a device whose browser has no PDF viewer. */
  readonly notViewableOnDevice: string;
  /** Noun phrase with article for a failed load, e.g. „die Datei“. */
  readonly fileObject: string;
  /** "2 von 5". */
  readonly position: (current: number, total: number) => string;
}

const GERMAN_LABELS: AttachmentViewerLabels = {
  close: "Schließen",
  previous: "Vorheriger Anhang",
  next: "Nächster Anhang",
  download: "Herunterladen",
  openInNewTab: "In neuem Tab öffnen",
  notViewable:
    "Diese Datei lässt sich hier nicht anzeigen. Laden Sie sie herunter, um sie zu öffnen.",
  notViewableOnDevice:
    "Auf diesem Gerät lässt sich die Datei hier nicht anzeigen. Öffnen Sie sie in einem neuen Tab oder laden Sie sie herunter.",
  fileObject: "die Datei",
  position: (current, total) => `${current} von ${total}`,
};

/** Pictures the browser shows itself; the upload accepts only these two. */
export function isViewerImage(contentType: string): boolean {
  return contentType === "image/png" || contentType === "image/jpeg";
}

function isPdf(contentType: string): boolean {
  return contentType === "application/pdf";
}

export function formatAttachmentSize(bytes: number): string {
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

/** "PDF", "Word", … for the file types a message carries. */
export function fileKindLabel(contentType: string): string | null {
  return (
    KIND_LABELS.find(([needle]) => contentType.includes(needle))?.[1] ?? null
  );
}

function fileMeta(item: ViewerItem): string {
  const kind = fileKindLabel(item.contentType);
  const size =
    item.sizeBytes === undefined ? null : formatAttachmentSize(item.sizeBytes);
  return [kind, size].filter(Boolean).join(" · ");
}

/**
 * Full-screen viewer for everything a person opens from a message: one item
 * at a time on a dark ground, with its name, a download action and, for
 * several items, previous and next. Pictures show as pictures, a PDF in the
 * browser's own viewer, every other file (Word, Excel, PowerPoint) as a
 * panel that offers the download. Arrow keys page, Escape closes, focus
 * stays inside while it is open.
 *
 * The kit's `Modal` is a white form panel with a heading row; a photo needs
 * the whole screen and no chrome around it, so this is its own overlay
 * (ADR 0149: one viewer for every picture or file a person opens).
 */
export function AttachmentViewer({
  items,
  index,
  onIndexChange,
  onClose,
  loadFile,
  labels = GERMAN_LABELS,
}: Readonly<{
  items: readonly ViewerItem[];
  /** The item shown; `null` keeps the viewer closed. */
  index: number | null;
  onIndexChange: (index: number) => void;
  onClose: () => void;
  /**
   * Loads the bytes of a PDF whose `src` is not a `blob:` URL. The app's
   * pages refuse to be framed, so a PDF shows from an object URL of bytes
   * the page loaded itself (#5692). Without it such a PDF offers the new
   * tab and the download only.
   */
  loadFile?: (src: string) => Promise<Blob>;
  labels?: AttachmentViewerLabels;
}>) {
  const open = index !== null && items[index] !== undefined;
  useScrollLock(open);

  const step = useCallback(
    (delta: number) => {
      if (index === null || items.length < 2) return;
      onIndexChange((index + delta + items.length) % items.length);
    },
    [index, items.length, onIndexChange],
  );

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
      if (event.key === "ArrowLeft") step(-1);
      if (event.key === "ArrowRight") step(1);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose, step]);

  if (!open || index === null) return null;
  const item = items[index];
  if (!item || typeof document === "undefined") return null;
  const several = items.length > 1;
  const downloadHref = item.downloadHref ?? item.src;

  return createPortal(
    <FocusScope asChild loop trapped>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={item.name}
        className={`fixed inset-0 ${Z_LAYER.modal} flex flex-col bg-black/90 text-white backdrop-blur-sm`}
        style={{ pointerEvents: "auto" }}
      >
        <div className="flex items-center gap-2 px-4 pt-[max(0.75rem,env(safe-area-inset-top))] pb-3">
          <p className="min-w-0 flex-1 truncate text-sm font-medium">
            {item.name}
          </p>
          {several ? (
            <p className="shrink-0 text-sm text-white/70 tabular-nums">
              {labels.position(index + 1, items.length)}
            </p>
          ) : null}
          <a
            href={downloadHref}
            download={item.name}
            aria-label={`${labels.download}: ${item.name}`}
            className={buttonClassName({
              variant: "ghost_tone",
              size: "icon",
              className: "",
            })}
            data-icon-only=""
          >
            <Download className="h-5 w-5" aria-hidden="true" />
          </a>
          <Button
            type="button"
            variant="ghost_tone"
            size="icon"
            onClick={onClose}
            aria-label={labels.close}
            tooltip={labels.close}
          >
            <CloseIcon />
          </Button>
        </div>
        {/* The ground closes the viewer; the item and the arrows do not. */}
        <div className="relative flex min-h-0 flex-1 items-center justify-center px-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
          <button
            type="button"
            tabIndex={-1}
            aria-label={labels.close}
            onClick={onClose}
            className="absolute inset-0 cursor-default"
          />
          <ViewerBody
            key={item.src}
            item={item}
            downloadHref={downloadHref}
            loadFile={loadFile}
            labels={labels}
          />
          {several ? (
            <>
              <div className="absolute top-1/2 left-2 -translate-y-1/2 sm:left-4">
                <Button
                  type="button"
                  variant="ghost_tone"
                  size="icon"
                  onClick={() => step(-1)}
                  aria-label={labels.previous}
                  tooltip={labels.previous}
                >
                  <ChevronLeft className="h-6 w-6" aria-hidden="true" />
                </Button>
              </div>
              <div className="absolute top-1/2 right-2 -translate-y-1/2 sm:right-4">
                <Button
                  type="button"
                  variant="ghost_tone"
                  size="icon"
                  onClick={() => step(1)}
                  aria-label={labels.next}
                  tooltip={labels.next}
                >
                  <ChevronRight className="h-6 w-6" aria-hidden="true" />
                </Button>
              </div>
            </>
          ) : null}
        </div>
      </div>
    </FocusScope>,
    document.body,
  );
}

interface BodyProps {
  readonly item: ViewerItem;
  readonly downloadHref: string;
  readonly loadFile?: (src: string) => Promise<Blob>;
  readonly labels: AttachmentViewerLabels;
}

function ViewerBody({ item, downloadHref, loadFile, labels }: BodyProps) {
  if (isViewerImage(item.contentType)) {
    return (
      // eslint-disable-next-line @next/next/no-img-element -- private, session-bound file; next/image cannot fetch it
      <img
        src={item.src}
        alt={item.name}
        className="relative max-h-full max-w-full rounded-lg object-contain shadow-2xl"
      />
    );
  }
  if (isPdf(item.contentType)) {
    return (
      <PdfBody
        item={item}
        downloadHref={downloadHref}
        loadFile={loadFile}
        labels={labels}
      />
    );
  }
  return (
    <FilePanel
      item={item}
      downloadHref={downloadHref}
      text={labels.notViewable}
      labels={labels}
    />
  );
}

/**
 * A PDF in the browser's viewer, from an object URL: the app answers every
 * frame of its own pages with `X-Frame-Options: DENY`, and its CSP frames
 * only `blob:` (proxy.ts). Below it the new tab and the download stay in
 * reach, for a phone whose frame shows only the first page.
 */
function PdfBody({ item, downloadHref, loadFile, labels }: BodyProps) {
  const viewer = usePdfViewerEnabled();
  const local = item.src.startsWith("blob:");
  const [blob, setBlob] = useState<Blob | null>(null);
  const [attempt, setAttempt] = useState(0);
  const { error, show, clear } = useApiLoadError();
  const loadedUrl = useObjectUrl(blob);
  const canShow = viewer && (local || loadFile !== undefined);

  useEffect(() => {
    if (!canShow || local || !loadFile) return;
    let current = true;
    clear();
    loadFile(item.src).then(
      (bytes) => {
        // Shown as a PDF whatever type the answer named, so nothing else
        // ever renders on this origin.
        if (current) setBlob(new Blob([bytes], { type: "application/pdf" }));
      },
      (err: unknown) => {
        if (!current) return;
        logger.warn("attachment_viewer_pdf_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void show(err, {
          object: labels.fileObject,
          retry: () => setAttempt((count) => count + 1),
        });
      },
    );
    return () => {
      current = false;
    };
  }, [
    canShow,
    local,
    loadFile,
    item.src,
    attempt,
    clear,
    show,
    labels.fileObject,
  ]);

  if (!canShow) {
    return (
      <FilePanel
        item={item}
        downloadHref={downloadHref}
        text={labels.notViewableOnDevice}
        openHref={item.src}
        labels={labels}
      />
    );
  }
  const url = local ? item.src : loadedUrl;
  return (
    <div className="pointer-events-none relative flex h-full w-full max-w-3xl flex-col items-center gap-3">
      {error ? (
        <div className="pointer-events-auto w-full max-w-md rounded-xl bg-white p-4 text-gray-900">
          <LoadErrorAlert error={error} />
        </div>
      ) : (
        <PdfPreview
          url={url}
          title={item.name}
          className="pointer-events-auto aspect-auto min-h-0 flex-1"
        />
      )}
      <div className="pointer-events-auto flex flex-wrap justify-center gap-2">
        <a
          href={url ?? item.src}
          target="_blank"
          rel="noopener noreferrer"
          className={buttonClassName({
            variant: "surface",
            size: "compact",
            className: "gap-1.5",
          })}
        >
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
          {labels.openInNewTab}
        </a>
        <a
          href={downloadHref}
          download={item.name}
          className={buttonClassName({
            variant: "surface",
            size: "compact",
            className: "gap-1.5",
          })}
        >
          <Download className="h-4 w-4" aria-hidden="true" />
          {labels.download}
        </a>
      </div>
    </div>
  );
}

/** A file the viewer cannot show: what it is, and its download. */
function FilePanel({
  item,
  downloadHref,
  text,
  openHref,
  labels,
}: Readonly<{
  item: ViewerItem;
  downloadHref: string;
  text: string;
  /** Offers the new tab too, for a PDF on a device without a viewer. */
  openHref?: string;
  labels: AttachmentViewerLabels;
}>) {
  const meta = fileMeta(item);
  return (
    <div className="relative flex w-full max-w-sm flex-col items-center rounded-2xl bg-white p-6 text-center text-gray-900 shadow-2xl">
      <IconTile
        size="display"
        icon={<FileTypeIcon contentType={item.contentType} className="" />}
      />
      <p className="mt-4 text-base font-semibold break-words">{item.name}</p>
      {meta ? <p className="mt-1 text-xs text-gray-500">{meta}</p> : null}
      <p className="mt-4 text-sm text-gray-600">{text}</p>
      <a
        href={downloadHref}
        download={item.name}
        className={buttonClassName({
          variant: "primary",
          size: "md",
          className: "mt-5 gap-2",
        })}
      >
        <Download className="h-4 w-4" aria-hidden="true" />
        {labels.download}
      </a>
      {openHref ? (
        <a
          href={openHref}
          target="_blank"
          rel="noopener noreferrer"
          className={buttonClassName({
            variant: "surface",
            size: "md",
            className: "mt-2 gap-2",
          })}
        >
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
          {labels.openInNewTab}
        </a>
      ) : null}
    </div>
  );
}
