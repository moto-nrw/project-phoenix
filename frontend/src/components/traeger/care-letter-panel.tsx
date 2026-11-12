"use client";

// Das Original einer Monatsrechnung, eines Stornos oder Jahresbescheids im
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
import type { BillingFile } from "~/lib/traeger-billing-api";
import type { CareLetterMailing } from "~/lib/traeger-care-billing-api";
import { formatEuro } from "./contribution-format";
import { LetterFileView } from "./letter-file-view";

type CareLetterDetails = Pick<
  CareLetterMailing,
  | "recipientFirstName"
  | "recipientLastName"
  | "childFirstName"
  | "childLastName"
  | "totalCents"
>;

export function CareLetterPanel<T extends CareLetterDetails>({
  letter,
  title,
  fileKey,
  load,
  onClose,
}: Readonly<{
  /** The letter to show; null closes the panel. */
  letter: T | null;
  title: string;
  fileKey: string;
  load: (letter: T) => Promise<BillingFile>;
  onClose: () => void;
}>) {
  return (
    // oxlint-disable-next-line ui-kit/slide-over-pending -- zeigt das Schreiben an und lädt es herunter; schreibt nichts.
    <SlideOver
      open={letter !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      {letter ? (
        <SlideOverContent size="lg">
          <SlideOverHeader>
            <SlideOverTitle>{title}</SlideOverTitle>
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
              fileKey={fileKey}
              load={() => load(letter)}
              title={title}
            />
          </SlideOverBody>
        </SlideOverContent>
      ) : null}
    </SlideOver>
  );
}
