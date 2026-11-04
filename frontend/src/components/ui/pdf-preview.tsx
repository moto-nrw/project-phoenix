"use client";

// Ein PDF in der Seite ansehen (#5692): der Rahmen im Format einer A4-Seite
// mit dem PDF-Viewer des Browsers. Die Datei kommt als Blob über den
// angemeldeten Endpunkt, nie als öffentliche oder signierte Adresse; die
// Object-URL lebt nur, solange das Blob gezeigt wird.

import { FileText } from "lucide-react";
import { useEffect, useState, useSyncExternalStore } from "react";
import { cn } from "~/lib/utils";
import { Skeleton } from "./skeleton";

/**
 * A short-lived object URL for the blob, released when the blob changes or
 * the component unmounts. Null while there is no blob.
 */
export function useObjectUrl(blob: Blob | null): string | null {
  const [entry, setEntry] = useState<{ blob: Blob; url: string } | null>(null);
  useEffect(() => {
    if (!blob) return;
    const url = URL.createObjectURL(blob);
    setEntry({ blob, url });
    return () => URL.revokeObjectURL(url);
  }, [blob]);
  // A URL of the previous blob is already revoked; never hand it out.
  return entry && entry.blob === blob ? entry.url : null;
}

const noSubscription = () => () => {};

/**
 * Whether the browser shows a PDF inside the page. Phone browsers without a
 * viewer report false (`navigator.pdfViewerEnabled`); the server render and
 * browsers without the property count as able.
 */
function usePdfViewerEnabled(): boolean {
  return useSyncExternalStore(
    noSubscription,
    () => globalThis.navigator?.pdfViewerEnabled !== false,
    () => true,
  );
}

/** The frame of one A4 page with the PDF, a skeleton while it loads. */
export function PdfPreview({
  url,
  title,
  className,
}: Readonly<{
  /** The object URL of the PDF; null while it loads. */
  url: string | null;
  /** Accessible name of the frame, e.g. „Rechnung FE-2026-0001“. */
  title: string;
  className?: string;
}>) {
  const viewer = usePdfViewerEnabled();
  return (
    <div
      className={cn(
        "relative aspect-[210/297] w-full overflow-hidden rounded-xl border border-gray-200 bg-gray-50",
        className,
      )}
    >
      {url === null ? (
        <div className="absolute inset-0 space-y-3 p-8" aria-hidden="true">
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-3 w-1/2" />
          <Skeleton className="mt-10 h-3 w-full" />
          <Skeleton className="h-3 w-11/12" />
          <Skeleton className="h-3 w-4/5" />
          <Skeleton className="mt-8 h-24 w-full" />
        </div>
      ) : viewer ? (
        <iframe
          key={url}
          src={`${url}#view=FitH`}
          title={title}
          className="absolute inset-0 h-full w-full bg-white"
        />
      ) : (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-6 text-center">
          <FileText className="h-10 w-10 text-gray-400" aria-hidden="true" />
          <p className="max-w-xs text-sm text-gray-600">
            Dieser Browser zeigt PDFs nicht in der Seite. Öffnen Sie die Datei
            in einem neuen Tab oder laden Sie sie herunter.
          </p>
        </div>
      )}
    </div>
  );
}
