"use client";

// Ein gespeichertes Schreiben als PDF (#5692), für die Schreiben der
// Ferienbetreuung und der Monatsbeiträge (#4684): „Herunterladen“, „In
// neuem Tab öffnen“ und die Seite selbst. Beide Knöpfe nehmen genau die
// gezeigte Datei, ohne zweite Anfrage. Jedes Öffnen geht über denselben
// angemeldeten Endpunkt wie der Download und steht deshalb im Protokoll.

import { Download, ExternalLink } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button, buttonClassName } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { PdfPreview, useObjectUrl } from "~/components/ui/pdf-preview";
import { useApiLoadError } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import type { BillingFile } from "~/lib/traeger-billing-api";

const logger = createLogger({ component: "LetterFileView" });

export function LetterFileView({
  fileKey,
  load,
  title,
}: Readonly<{
  /** Names the file; a new key loads it again. */
  fileKey: string;
  load: () => Promise<BillingFile>;
  /** „Rechnung FE-2026-0001“: names the frame and both buttons. */
  title: string;
}>) {
  // The file is kept with the key it belongs to: while the next one loads,
  // the frame never shows the previous one under the new heading.
  const [loaded, setLoaded] = useState<{
    key: string;
    file: BillingFile;
  } | null>(null);
  const [attempt, setAttempt] = useState(0);
  const {
    error,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const file = loaded?.key === fileKey ? loaded.file : null;
  const url = useObjectUrl(file?.blob ?? null);

  // `load` belongs to the key: a new function for the same key loads
  // nothing, so callers may pass it inline.
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  }, [load]);

  useEffect(() => {
    let current = true;
    clearLoadError();
    loadRef.current().then(
      (next) => {
        if (!current) return;
        // The frame and the tab show the file as a PDF whatever type the
        // response named, so nothing else ever renders on this origin.
        const blob = new Blob([next.blob], { type: "application/pdf" });
        setLoaded({ key: fileKey, file: { blob, filename: next.filename } });
      },
      (err: unknown) => {
        if (!current) return;
        logger.error("traeger_letter_file_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showLoadError(err, {
          object: "das Schreiben",
          retry: () => setAttempt((count) => count + 1),
        });
      },
    );
    return () => {
      current = false;
    };
  }, [fileKey, attempt, clearLoadError, showLoadError]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="surface"
          size="compact"
          disabled={!file}
          aria-label={`${title} herunterladen`}
          onClick={() => {
            if (file) downloadBlob(file.blob, file.filename);
          }}
          className="gap-1.5"
        >
          <Download className="h-4 w-4" aria-hidden="true" />
          Herunterladen
        </Button>
        {url ? (
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`${title} in neuem Tab öffnen`}
            className={buttonClassName({
              variant: "surface",
              size: "compact",
              className: "gap-1.5",
            })}
          >
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
            In neuem Tab öffnen
          </a>
        ) : (
          <Button
            type="button"
            variant="surface"
            size="compact"
            disabled
            className="gap-1.5"
          >
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
            In neuem Tab öffnen
          </Button>
        )}
      </div>
      {error ? (
        <LoadErrorAlert error={error} actionLayout="responsive" />
      ) : (
        <PdfPreview url={url} title={title} />
      )}
    </div>
  );
}
