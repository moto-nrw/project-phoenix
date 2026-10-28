"use client";

// Eine Rechnung im Abrechnungslauf (#4668): je Kind die Positionen aus dem
// festgehaltenen Ferienbeitrag und die Korrekturen mit Begründung. Vor der
// Freigabe ein Entwurf ohne Nummer, danach die ausgestellte Rechnung. Seit
// #5146 steht sie im Seitenpanel, geöffnet aus dem Menü ihrer Zeile in der
// Rechnungstabelle (BillingRunOverviewCard), statt als eigene Karte.

import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import type { BillingChild, BillingDocument } from "~/lib/traeger-billing-api";
import { billingPositionLabel } from "./billing-format";
import { formatEuro } from "./contribution-format";
import { formatDateRange } from "./holiday-period-format";

export function recipientName(document: BillingDocument): string {
  return `${document.recipientFirstName} ${document.recipientLastName}`;
}

function issuedText(document: BillingDocument): string {
  return document.number
    ? `Rechnung ${document.number}${document.issuedOn ? ` vom ${formatDate(document.issuedOn)}` : ""}`
    : "Entwurf ohne Nummer";
}

function CorrectionHistory({ child }: Readonly<{ child: BillingChild }>) {
  if (child.corrections.length === 0) return null;
  return (
    <div className="space-y-1">
      <p className="text-sm font-medium text-gray-700">
        Korrekturen für {child.firstName}
      </p>
      <ul className="space-y-1 text-sm text-gray-700">
        {child.corrections.map((correction) => (
          <li key={correction.id}>
            {formatBerlinDate(correction.createdAt)}:{" "}
            <span className="tabular-nums">
              {formatEuro(correction.totalBeforeCents)} →{" "}
              {formatEuro(correction.totalAfterCents)}
            </span>
            . {correction.reason}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** Die Aufschlüsselung einer Rechnung: Positionen je Kind und Korrekturen. */
export function BillingDocumentBreakdown({
  document,
  onClose,
}: Readonly<{ document: BillingDocument | null; onClose: () => void }>) {
  return (
    <SlideOver
      open={document !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      {document ? (
        <SlideOverContent>
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <SlideOverTitle>
                Rechnung an {recipientName(document)}
              </SlideOverTitle>
              <SlideOverDescription>
                {issuedText(document)} · {document.recipientEmail} · Betreuung{" "}
                {formatDateRange(document.serviceFrom, document.serviceUntil)}
              </SlideOverDescription>
            </div>
            <SlideOverCloseButton />
          </SlideOverHeader>
          <SlideOverBody className="space-y-4">
            <ContributionBreakdown
              items={document.children.map((child) => ({
                key: child.childId,
                name: `${child.firstName} ${child.lastName}`,
                lines: child.positions.map((position) => ({
                  label: billingPositionLabel(position),
                  amount: formatEuro(position.amountCents),
                })),
                total: formatEuro(child.totalCents),
              }))}
              childTotalLabel="Summe"
              totalLabel="Rechnungsbetrag"
              total={formatEuro(document.totalCents)}
            />
            {document.children.map((child) => (
              <CorrectionHistory key={child.childId} child={child} />
            ))}
          </SlideOverBody>
        </SlideOverContent>
      ) : null}
    </SlideOver>
  );
}
