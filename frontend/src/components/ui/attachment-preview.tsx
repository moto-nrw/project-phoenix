"use client";

import { X } from "lucide-react";

import { cn } from "~/lib/utils";

import { FileTypeIcon } from "./file-type-icon";
import { SpinnerIcon } from "./icons";

/**
 * A file somebody has picked and not sent yet, as it sits in a composer
 * (#6685): a picture shows itself, any other file its type, name and size.
 * While `busy` it is still on its way to the server. The cross takes it out
 * again; under a finger its hit area reaches 44 px. With `onOpen` the tile
 * itself is a button that opens a preview.
 *
 * Tile and cross are concentric: the tile is `rounded-xl` (16 px), the round
 * cross sits 4 px inside its corner (radius 12 px). Nothing hangs over the
 * edge, so a scrolling tray or a rounded frame cannot clip it.
 */
export function AttachmentPreview({
  name,
  detail,
  contentType,
  previewUrl,
  busy = false,
  busyLabel,
  onRemove,
  removeLabel,
  onOpen,
  openLabel,
}: Readonly<{
  name: string;
  /** Quiet second line of a file, for example its size. */
  detail?: string;
  contentType: string;
  /** Address of the picture itself; without it the file shows as a row. */
  previewUrl?: string;
  busy?: boolean;
  /** Read out while `busy`, for example „foto.png wird hochgeladen“. */
  busyLabel: string;
  onRemove: () => void;
  removeLabel: string;
  /** Opens a preview of the file; the tile body becomes a button. */
  onOpen?: () => void;
  /** Name of that button, for example „Anhang öffnen: foto.png“. */
  openLabel?: string;
}>) {
  // Dasselbe Kreuz wie am `RemovableChip`, auf einer kleinen weißen Marke in
  // der Ecke, damit es auch auf einem Foto zu sehen ist. Die Trefferfläche
  // wächst vor allem nach außen (oben, rechts) in den Rand der Leiste, damit
  // vom Foto möglichst viel zum Öffnen bleibt.
  const remove = (
    <span className="absolute top-1 right-1 flex rounded-full bg-white/90 shadow-sm ring-1 ring-black/5">
      <button
        type="button"
        onClick={onRemove}
        aria-label={removeLabel}
        className="relative flex size-6 items-center justify-center rounded-full text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none pointer-coarse:after:absolute pointer-coarse:after:-top-3 pointer-coarse:after:-right-3 pointer-coarse:after:-bottom-2 pointer-coarse:after:-left-2"
      >
        <X className="size-3.5" aria-hidden="true" />
      </button>
    </span>
  );
  const status = busy ? (
    <span role="status" aria-label={busyLabel} className="flex">
      <SpinnerIcon className="h-4 w-4 motion-reduce:animate-none" />
    </span>
  ) : null;
  const openFocus =
    "transition-opacity focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";

  if (previewUrl) {
    const picture = (
      // eslint-disable-next-line @next/next/no-img-element -- Vorschau aus dem Gerät (blob:), kein Server-Bild
      <img
        src={previewUrl}
        alt={name}
        className="size-16 rounded-xl object-cover outline outline-1 -outline-offset-1 outline-black/10"
      />
    );
    return (
      <span className="relative block size-16 shrink-0">
        {onOpen ? (
          <button
            type="button"
            onClick={onOpen}
            aria-label={openLabel}
            className={cn(
              "block size-16 rounded-xl hover:opacity-90",
              openFocus,
            )}
          >
            {picture}
          </button>
        ) : (
          picture
        )}
        {busy ? (
          <span className="pointer-events-none absolute inset-0 flex items-center justify-center rounded-xl bg-white/70 text-gray-700">
            {status}
          </span>
        ) : null}
        {remove}
      </span>
    );
  }

  const body = (
    <>
      {busy ? (
        <span className="text-gray-500">{status}</span>
      ) : (
        <FileTypeIcon
          contentType={contentType}
          className="h-5 w-5 shrink-0 text-gray-500"
        />
      )}
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-sm font-medium text-gray-900">
          {name}
        </span>
        {detail ? (
          <span className="truncate text-xs text-gray-500">{detail}</span>
        ) : null}
      </span>
    </>
  );
  // Rechts bleibt Platz für das Kreuz, damit der Name nicht darunter läuft.
  const bodyLayout =
    "flex h-full min-w-0 items-center gap-2.5 rounded-xl py-2 pr-9 pl-3 text-left";
  return (
    <span className="relative flex h-16 max-w-44 min-w-0 shrink-0 rounded-xl bg-gray-50 ring-1 ring-gray-200 ring-inset sm:max-w-56">
      {onOpen ? (
        <button
          type="button"
          onClick={onOpen}
          aria-label={openLabel}
          // Durchscheinende Tönung: der Rahmen der Kachel bleibt sichtbar.
          className={cn(bodyLayout, "hover:bg-gray-900/5", openFocus)}
        >
          {body}
        </button>
      ) : (
        <span className={bodyLayout}>{body}</span>
      )}
      {remove}
    </span>
  );
}
