"use client";

import { ArrowLeft, Download } from "lucide-react";
import { ButtonLink, buttonClassName } from "~/components/ui/button";

/**
 * Kopf der gedruckten NFC-Kurzanleitung.
 *
 * Diese Seite ist der letzte Rest des alten Guide-Systems: sie ist kein
 * Hilfe-Artikel, sondern die Vorlage fuer das Blatt im Versandkarton. CI
 * rendert sie mit `pnpm run generate:guides` nach
 * `public/help/pdfs/nfc-erste-schritte.pdf`.
 *
 * Fruehere Fassung nutzte `HelpHeader` aus `guide-components.tsx`. Mit dem
 * Wechsel auf die Themenhilfe fiel die Datei weg, und der Kopf zog hierher --
 * ohne die Guide-Suche, die es nicht mehr gibt.
 *
 * `print:hidden`: im PDF waere ein Knopf zum Herunterladen des PDFs sinnlos.
 */
export function OnepagerHeader({
  pdf,
}: {
  readonly pdf: { readonly href: string; readonly download: string };
}) {
  return (
    <header className="sticky top-3 z-30 print:hidden">
      <div className="moto-content-surface flex items-center justify-between gap-3 rounded-2xl border p-3 shadow-sm sm:relative sm:p-4">
        <ButtonLink
          href="/help"
          variant="surface"
          size="md"
          className="min-w-0 gap-2"
        >
          <ArrowLeft className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="truncate">Zur Hilfe</span>
        </ButtonLink>

        <div className="flex min-h-10 w-fit shrink-0 items-center">
          <a
            href={pdf.href}
            download={pdf.download}
            className={buttonClassName({
              variant: "surface",
              size: "md",
              className: "gap-2 whitespace-nowrap",
            })}
            aria-label="PDF herunterladen"
          >
            <Download className="h-4 w-4" aria-hidden="true" />
            <span className="hidden sm:inline">PDF herunterladen</span>
          </a>
        </div>
      </div>
    </header>
  );
}
