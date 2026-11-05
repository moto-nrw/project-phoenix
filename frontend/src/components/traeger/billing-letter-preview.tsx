"use client";

// Das Schreiben einer Rechnung als PDF neben der Liste (#5692). Es ist
// dieselbe Datei wie „Schreiben herunterladen“, geladen über denselben
// angemeldeten Endpunkt; jedes Öffnen steht deshalb wie ein Download im
// Protokoll des Laufs. Kopf mit Nummer, Empfänger und Betrag, damit die
// Auswahl in der Liste und das Dokument eindeutig zusammengehören.
// „Herunterladen“ und „In neuem Tab öffnen“ nehmen genau die gezeigte Datei,
// ohne zweite Anfrage.

import { Download, ExternalLink, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { BackAction } from "~/components/ui/back-link";
import { Button, buttonClassName } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { PdfPreview, useObjectUrl } from "~/components/ui/pdf-preview";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingDocument,
  type BillingFile,
  downloadBillingLetter,
} from "~/lib/traeger-billing-api";
import { recipientName } from "./billing-document-card";
import { BILLING_DOCUMENT_KIND_LABELS, isCancelled } from "./billing-format";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "BillingLetterPreview" });

/** „Rechnung FE-2026-0001“, „Storno FE-2026-0007“. */
function billingDocumentTitle(document: BillingDocument): string {
  return `${BILLING_DOCUMENT_KIND_LABELS[document.kind]} ${document.number}`;
}

export function BillingLetterPreview({
  runId,
  document,
  onClose,
}: Readonly<{
  runId: string;
  document: BillingDocument;
  /** Leaves the preview: back to the list on a phone, deselect on a desk. */
  onClose: () => void;
}>) {
  // The file is kept with the ID it belongs to: while the next document
  // loads, the frame never shows the previous one under the new heading.
  const [loaded, setLoaded] = useState<{
    documentId: string;
    file: BillingFile;
  } | null>(null);
  const [attempt, setAttempt] = useState(0);
  const {
    error,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const documentId = document.id;
  const file = loaded?.documentId === documentId ? loaded.file : null;
  const url = useObjectUrl(file?.blob ?? null);
  const title = billingDocumentTitle(document);
  const headingRef = useRef<HTMLHeadingElement>(null);

  // Below xl the chosen row is hidden with the list: the reader continues
  // at the heading of the letter, on a desk too.
  useEffect(() => {
    headingRef.current?.focus({ preventScroll: true });
  }, [documentId]);

  useEffect(() => {
    let current = true;
    clearLoadError();
    downloadBillingLetter(runId, documentId).then(
      (next) => {
        if (!current) return;
        // The frame and the tab show the file as a PDF whatever type the
        // response named, so nothing else ever renders on this origin.
        const blob = new Blob([next.blob], { type: "application/pdf" });
        setLoaded({ documentId, file: { blob, filename: next.filename } });
      },
      (err: unknown) => {
        if (!current) return;
        logger.error("traeger_billing_letter_preview_failed", {
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
  }, [runId, documentId, attempt, clearLoadError, showLoadError]);

  return (
    <section aria-label={`Vorschau: ${title}`} className="space-y-4">
      <BackAction
        onClick={onClose}
        label="Zurück zu den Rechnungen"
        className="xl:hidden"
      />
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3
              ref={headingRef}
              tabIndex={-1}
              className="text-base font-semibold text-gray-900 outline-none"
            >
              {BILLING_DOCUMENT_KIND_LABELS[document.kind]}{" "}
              <span className="font-mono">{document.number}</span>
            </h3>
            {isCancelled(document) ? (
              <StatusBadge label="Storniert" tone="red" />
            ) : null}
          </div>
          <p className="text-sm text-gray-600">
            {recipientName(document)}
            <span aria-hidden="true"> · </span>
            <span className="sr-only">, </span>
            <span className="tabular-nums">
              {formatEuro(document.totalCents)}
            </span>
          </p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Vorschau schließen"
          onClick={onClose}
          className="hidden shrink-0 xl:inline-flex"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
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
    </section>
  );
}
