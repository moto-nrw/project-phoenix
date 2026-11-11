"use client";

import { X } from "lucide-react";

import { FileTypeIcon } from "./file-type-icon";
import { SpinnerIcon } from "./icons";

/**
 * A file somebody has picked and not sent yet, as it sits in a composer
 * (#6685): a picture shows itself, any other file its type, name and size.
 * While `busy` it is still on its way to the server. The cross takes it out
 * again; under a finger its hit area reaches 44 px.
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
}>) {
  // Dasselbe Kreuz wie am `RemovableChip`, auf einer kleinen weißen Marke an
  // der Ecke, damit es auch auf einem Foto zu sehen ist.
  const remove = (
    <span className="absolute -top-1.5 -right-1.5 flex rounded-full bg-white shadow-sm ring-1 ring-gray-200">
      <button
        type="button"
        onClick={onRemove}
        aria-label={removeLabel}
        className="relative flex h-5 w-5 items-center justify-center rounded-full text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none pointer-coarse:after:absolute pointer-coarse:after:-inset-3"
      >
        <X className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
    </span>
  );
  const status = busy ? (
    <span role="status" aria-label={busyLabel} className="flex">
      <SpinnerIcon className="h-4 w-4 motion-reduce:animate-none" />
    </span>
  ) : null;

  if (previewUrl) {
    return (
      <span className="relative block h-16 w-16 shrink-0">
        {/* eslint-disable-next-line @next/next/no-img-element -- Vorschau aus dem Gerät (blob:), kein Server-Bild */}
        <img
          src={previewUrl}
          alt={name}
          className="h-16 w-16 rounded-xl object-cover outline outline-1 -outline-offset-1 outline-black/10"
        />
        {busy ? (
          <span className="absolute inset-0 flex items-center justify-center rounded-xl bg-white/70 text-gray-700">
            {status}
          </span>
        ) : null}
        {remove}
      </span>
    );
  }
  return (
    <span className="relative flex h-16 max-w-56 min-w-0 shrink-0 items-center gap-2.5 rounded-xl bg-gray-50 py-2 pr-4 pl-3 ring-1 ring-gray-200 ring-inset">
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
      {remove}
    </span>
  );
}
