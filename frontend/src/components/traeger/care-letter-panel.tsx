"use client";

// Das Schreiben einer Monatsrechnung oder ihres Stornos als PDF im
// Seitenpanel (#4684), geöffnet aus dem Menü ihrer Zeile. Es ist dieselbe
// Datei, die die Familie bekommt.

import {
  SlideOver,
  SlideOverBody,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import {
  type CareLetterMailing,
  downloadCareLetter,
} from "~/lib/traeger-care-billing-api";
import { formatEuro } from "./contribution-format";
import { LetterFileView } from "./letter-file-view";

/** „Rechnung MB-2027-0001“, „Storno MB-2027-0009“. */
function careLetterTitle(letter: CareLetterMailing): string {
  return `${letter.kind === "cancellation" ? "Storno" : "Rechnung"} ${letter.number}`;
}

export function CareLetterPanel({
  runId,
  letter,
  onClose,
}: Readonly<{
  runId: string;
  /** The letter to show; null closes the panel. */
  letter: CareLetterMailing | null;
  onClose: () => void;
}>) {
  return (
    <SlideOver
      open={letter !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      {letter ? (
        <SlideOverContent size="lg">
          <SlideOverHeader>
            <SlideOverTitle>{careLetterTitle(letter)}</SlideOverTitle>
            <SlideOverDescription className="wrap-anywhere">
              An {letter.recipientFirstName} {letter.recipientLastName} · Für{" "}
              {letter.childFirstName} {letter.childLastName} ·{" "}
              <span className="tabular-nums">
                {formatEuro(letter.totalCents)}
              </span>
            </SlideOverDescription>
          </SlideOverHeader>
          <SlideOverBody>
            <LetterFileView
              fileKey={`${letter.kind}-${letter.invoiceId}`}
              load={() =>
                downloadCareLetter(runId, letter.invoiceId, letter.kind)
              }
              title={careLetterTitle(letter)}
            />
          </SlideOverBody>
        </SlideOverContent>
      ) : null}
    </SlideOver>
  );
}
