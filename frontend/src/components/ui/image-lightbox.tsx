"use client";

import { useCallback, useEffect } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import { ChevronLeft, ChevronRight, Download } from "lucide-react";

import { useScrollLock } from "~/components/ui/hooks/useScrollLock";
import { buttonClassName, Button } from "./button";
import { CloseIcon } from "./close-icon";
import { Z_LAYER } from "./overlay-styles";

/** One picture the viewer shows. */
export interface LightboxImage {
  readonly src: string;
  /** File name; shown as the caption and read as the picture's name. */
  readonly name: string;
  /** Address that saves the file. Omit to hide the download action. */
  readonly downloadHref?: string;
}

export interface ImageLightboxLabels {
  readonly close: string;
  readonly previous: string;
  readonly next: string;
  readonly download: string;
  /** "2 von 5". */
  readonly position: (current: number, total: number) => string;
}

const GERMAN_LABELS: ImageLightboxLabels = {
  close: "Schließen",
  previous: "Vorheriges Bild",
  next: "Nächstes Bild",
  download: "Herunterladen",
  position: (current, total) => `${current} von ${total}`,
};

/**
 * Full-screen picture viewer: one picture at a time on a dark ground, with
 * its name, a download action and, for several pictures, previous and next.
 * Arrow keys page, Escape closes, focus stays inside while it is open.
 *
 * The kit's `Modal` is a white form panel with a heading row; a photo needs
 * the whole screen and no chrome around it, so this is its own overlay
 * (ADR 0149: one viewer for every picture a person opens).
 */
export function ImageLightbox({
  images,
  index,
  onIndexChange,
  onClose,
  labels = GERMAN_LABELS,
}: Readonly<{
  images: readonly LightboxImage[];
  /** The picture shown; `null` keeps the viewer closed. */
  index: number | null;
  onIndexChange: (index: number) => void;
  onClose: () => void;
  labels?: ImageLightboxLabels;
}>) {
  const open = index !== null && images[index] !== undefined;
  useScrollLock(open);

  const step = useCallback(
    (delta: number) => {
      if (index === null || images.length < 2) return;
      onIndexChange((index + delta + images.length) % images.length);
    },
    [index, images.length, onIndexChange],
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
  const image = images[index];
  if (!image || typeof document === "undefined") return null;
  const several = images.length > 1;

  return createPortal(
    <FocusScope asChild loop trapped>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={image.name}
        className={`fixed inset-0 ${Z_LAYER.modal} flex flex-col bg-black/90 text-white backdrop-blur-sm`}
        style={{ pointerEvents: "auto" }}
      >
        <div className="flex items-center gap-2 px-4 pt-[max(0.75rem,env(safe-area-inset-top))] pb-3">
          <p className="min-w-0 flex-1 truncate text-sm font-medium">
            {image.name}
          </p>
          {several ? (
            <p className="shrink-0 text-sm text-white/70 tabular-nums">
              {labels.position(index + 1, images.length)}
            </p>
          ) : null}
          {image.downloadHref ? (
            <a
              href={image.downloadHref}
              download={image.name}
              aria-label={`${labels.download}: ${image.name}`}
              className={buttonClassName({
                variant: "ghost_tone",
                size: "icon",
                className: "",
              })}
              data-icon-only=""
            >
              <Download className="h-5 w-5" aria-hidden="true" />
            </a>
          ) : null}
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
        {/* The ground closes the viewer; the picture and the arrows do not. */}
        <div className="relative flex min-h-0 flex-1 items-center justify-center px-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
          <button
            type="button"
            tabIndex={-1}
            aria-label={labels.close}
            onClick={onClose}
            className="absolute inset-0 cursor-default"
          />
          {/* eslint-disable-next-line @next/next/no-img-element -- private, session-bound file; next/image cannot fetch it */}
          <img
            key={image.src}
            src={image.src}
            alt={image.name}
            className="relative max-h-full max-w-full rounded-lg object-contain shadow-2xl"
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
