"use client";

// Eine Rechnung im Abrechnungslauf (#4668): je Kind die Positionen aus dem
// festgehaltenen Ferienbeitrag und die Korrekturen mit Begründung. Vor der
// Freigabe ein Entwurf ohne Nummer, danach die ausgestellte Rechnung.

import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import type { BillingChild, BillingDocument } from "~/lib/traeger-billing-api";
import { billingPositionLabel } from "./billing-format";
import { formatEuro } from "./contribution-format";
import { formatDateRange } from "./holiday-period-format";

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

export function BillingDocumentCard({
  document,
  onCorrect,
  onDownloadLetter,
}: Readonly<{
  document: BillingDocument;
  /** Set while the run is a preview; corrects one child. */
  onCorrect?: (child: BillingChild) => void;
  /** Set once the letters of the released run exist (#4669). */
  onDownloadLetter?: () => void;
}>) {
  const recipient = `${document.recipientFirstName} ${document.recipientLastName}`;
  const issued = document.number
    ? `Rechnung ${document.number}${document.issuedOn ? ` vom ${formatDate(document.issuedOn)}` : ""}`
    : "Entwurf ohne Nummer";
  return (
    <SectionCard
      title={recipient}
      description={`${issued} · ${document.recipientEmail} · Betreuung ${formatDateRange(document.serviceFrom, document.serviceUntil)}`}
      actions={
        onCorrect || onDownloadLetter ? (
          <OverflowMenu
            ariaLabel={`Aktionen für die Rechnung an ${recipient}`}
            triggerSize="sm"
            items={[
              ...(onCorrect
                ? document.children.map((child) => ({
                    label: `Betrag für ${child.firstName} korrigieren`,
                    onClick: () => onCorrect(child),
                  }))
                : []),
              ...(onDownloadLetter
                ? [
                    {
                      label: "Schreiben herunterladen",
                      onClick: onDownloadLetter,
                    },
                  ]
                : []),
            ]}
          />
        ) : null
      }
    >
      <div className="space-y-4">
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
      </div>
    </SectionCard>
  );
}
