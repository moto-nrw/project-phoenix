"use client";

// Das Schreiben einer Rechnung als PDF neben der Liste (#5692). Es ist
// dieselbe Datei wie „Schreiben herunterladen“, geladen über denselben
// angemeldeten Endpunkt; jedes Öffnen steht deshalb wie ein Download im
// Protokoll des Laufs. Kopf mit Nummer, Empfänger und Betrag, damit die
// Auswahl in der Liste und das Dokument eindeutig zusammengehören.
// „Herunterladen“ und „In neuem Tab öffnen“ nehmen genau die gezeigte Datei,
// ohne zweite Anfrage.

import { X } from "lucide-react";
import { useEffect, useRef } from "react";
import { BackAction } from "~/components/ui/back-link";
import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  type BillingDocument,
  downloadBillingLetter,
} from "~/lib/traeger-billing-api";
import { recipientName } from "./billing-document-card";
import { BILLING_DOCUMENT_KIND_LABELS, isCancelled } from "./billing-format";
import { formatEuro } from "./contribution-format";
import { LetterFileView } from "./letter-file-view";

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
  const documentId = document.id;
  const title = billingDocumentTitle(document);
  const headingRef = useRef<HTMLHeadingElement>(null);

  // Below xl the chosen row is hidden with the list: the reader continues
  // at the heading of the letter, on a desk too.
  useEffect(() => {
    headingRef.current?.focus({ preventScroll: true });
  }, [documentId]);

  return (
    <section aria-label={`Schreiben: ${title}`} className="space-y-4">
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
          aria-label="Schreiben schließen"
          onClick={onClose}
          className="hidden shrink-0 xl:inline-flex"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
      <LetterFileView
        fileKey={documentId}
        load={() => downloadBillingLetter(runId, documentId)}
        title={title}
      />
    </section>
  );
}
