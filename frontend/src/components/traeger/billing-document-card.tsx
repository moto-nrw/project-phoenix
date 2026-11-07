"use client";

// Eine Rechnung im Abrechnungslauf (#4668): je Kind die Positionen aus dem
// festgehaltenen Ferienbeitrag und die Korrekturen mit Begründung. Vor der
// Freigabe ein Entwurf ohne Nummer, danach die ausgestellte Rechnung. Seit
// #5146 steht sie im Seitenpanel, geöffnet aus dem Menü ihrer Zeile in der
// Rechnungstabelle (BillingRunOverviewCard), statt als eigene Karte. Seit
// #4690 zeigt es auch Storno und Gutschrift mit Grund und Bezug und den
// Verlauf einer Rechnung: ihren Storno, ihre Gutschriften und die neue
// Rechnung, die sie ersetzt.

import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import {
  SlideOver,
  SlideOverBody,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { Button } from "~/components/ui/button";
import NavigationLink from "~/components/ui/navigation-link";
import { ContributionRulesLink } from "./contribution-rules-link";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import type { BillingChild, BillingDocument } from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";
import {
  BILLING_DOCUMENT_KIND_LABELS,
  billingPositionLabel,
  billingReferenceText,
  isCancelled,
} from "./billing-format";
import { formatEuro } from "./contribution-format";
import { formatDateRange } from "./holiday-period-format";

export function recipientName(document: BillingDocument): string {
  return `${document.recipientFirstName} ${document.recipientLastName}`;
}

function issuedText(document: BillingDocument): string {
  const reference = billingReferenceText(document);
  const issued = document.number
    ? `${BILLING_DOCUMENT_KIND_LABELS[document.kind]} ${document.number}${document.issuedOn ? ` vom ${formatDate(document.issuedOn)}` : ""}`
    : "Entwurf ohne Nummer";
  return reference ? `${issued} · ${reference}` : issued;
}

/**
 * The history of an issued invoice: its Storno, its Gutschriften and the
 * new invoice that replaces it. A cancelled invoice without a new one says
 * how the family gets it.
 */
function DocumentHistory({
  document,
  runId,
  onReissue,
}: Readonly<{
  document: BillingDocument;
  runId: string;
  onReissue?: () => void;
}>) {
  if (document.kind !== "invoice" || !document.number) return null;
  const reissued = document.history.some((link) => link.kind === "invoice");
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium text-gray-700">Verlauf</p>
      {document.history.length === 0 ? (
        <p className="text-sm text-gray-700">
          Kein Storno und keine Gutschrift.
        </p>
      ) : (
        <ul className="space-y-1 text-sm text-gray-700">
          {document.history.map((link) => (
            <li key={link.id} className="wrap-anywhere">
              {formatDate(link.issuedOn)}:{" "}
              {link.kind === "invoice" ? (
                <>
                  Neue Rechnung{" "}
                  {link.runId === runId ? (
                    <span className="font-mono">{link.number}</span>
                  ) : (
                    <NavigationLink
                      href={traegerPath(`/traeger/abrechnung/${link.runId}`)}
                      className="font-mono underline"
                    >
                      {link.number}
                    </NavigationLink>
                  )}{" "}
                  über{" "}
                  <span className="tabular-nums">
                    {formatEuro(link.totalCents)}
                  </span>
                </>
              ) : (
                <>
                  {BILLING_DOCUMENT_KIND_LABELS[link.kind]}{" "}
                  <span className="font-mono">{link.number}</span> über{" "}
                  <span className="tabular-nums">
                    {formatEuro(link.totalCents)}
                  </span>
                  . {link.reason}
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      {isCancelled(document) && !reissued ? (
        <div className="space-y-2">
          <p className="text-sm text-gray-700">
            Die Rechnung ist storniert. Die neue Rechnung stellen Sie in einem
            neuen Lauf aus. Dort steht die Familie in der Vorschau.
          </p>
          {onReissue ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={onReissue}
            >
              Neu ausstellen
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function CorrectionHistory({ child }: Readonly<{ child: BillingChild }>) {
  if (child.corrections.length === 0 && child.carriedCorrections.length === 0) {
    return null;
  }
  return (
    <div className="space-y-1">
      <p className="text-sm font-medium text-gray-700">
        Korrekturen für {child.firstName}
      </p>
      <ul className="space-y-1 text-sm text-gray-700">
        {child.carriedCorrections.map((carried) => (
          <li key={carried.positionId}>
            <span className="tabular-nums">
              {formatEuro(carried.amountCents)}
            </span>{" "}
            aus {carried.sourceNumber}. {carried.reason}
          </li>
        ))}
        {child.corrections.map((correction) => (
          <li key={correction.id}>
            {formatBerlinDate(correction.createdAt)}:{" "}
            <span className="tabular-nums">
              {formatEuro(correction.totalBeforeCents)} →{" "}
              {formatEuro(correction.totalAfterCents)}
            </span>
            {correction.carriedFromNumber
              ? ` aus ${correction.carriedFromNumber}`
              : ""}
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
  runId,
  onClose,
  onReissue,
}: Readonly<{
  document: BillingDocument | null;
  runId: string;
  onClose: () => void;
  /** Opens the run that issues a cancelled invoice again (#4690). */
  onReissue?: (document: BillingDocument) => void;
}>) {
  return (
    <SlideOver
      open={document !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      {document ? (
        <SlideOverContent>
          <SlideOverHeader>
            <SlideOverTitle>
              {BILLING_DOCUMENT_KIND_LABELS[document.kind]} an{" "}
              {recipientName(document)}
            </SlideOverTitle>
            {/* E-Mail und Debitorennummer (bis 35 Zeichen am Stück) brechen um. */}
            <SlideOverDescription className="wrap-anywhere">
              {issuedText(document)} · {document.recipientEmail}
              {document.debtorNumber
                ? ` · Debitorennummer ${document.debtorNumber}`
                : ""}{" "}
              · Betreuung{" "}
              {formatDateRange(document.serviceFrom, document.serviceUntil)}
            </SlideOverDescription>
          </SlideOverHeader>
          <SlideOverBody className="space-y-4">
            {document.reason ? (
              <p className="text-sm wrap-anywhere text-gray-700">
                Grund: {document.reason}
              </p>
            ) : null}
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
              totalLabel={
                document.kind === "invoice"
                  ? "Rechnungsbetrag"
                  : `${BILLING_DOCUMENT_KIND_LABELS[document.kind]} gesamt`
              }
              total={formatEuro(document.totalCents)}
            />
            <ContributionRulesLink
              // Der Abrechnungslauf setzt das Recht „Abrechnung“ voraus.
              canManageBillingSettings
              kinds={document.children.flatMap((child) =>
                child.positions.map((position) => position.kind),
              )}
            />
            {document.children.map((child) => (
              <CorrectionHistory key={child.childId} child={child} />
            ))}
            <DocumentHistory
              document={document}
              runId={runId}
              onReissue={onReissue ? () => onReissue(document) : undefined}
            />
          </SlideOverBody>
        </SlideOverContent>
      ) : null}
    </SlideOver>
  );
}
