"use client";

// Die Prüfphase eines Monatslaufs vor der Freigabe (#5916, SJR-Abgleich E1
// A): das Menü „Ausschließen“ je Rechnung, die Karte der ausgeschlossenen
// Rechnungen mit dem Weg zurück und die Prüfliste als Excel oder CSV. Eine
// ausgeschlossene Rechnung bekommt bei der Freigabe keine Nummer; ihr Kind
// kommt im nächsten Lauf desselben Monats wieder. Die Laufseite lädt diese
// Datei erst in der Prüfphase (care-run-review-lazy.tsx), die Dialoge erst,
// wenn jemand sie öffnet.

import { useState } from "react";
import { Button, ButtonLink } from "~/components/ui/button";
import {
  type ExportMenuOption,
  ExportMenu,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import {
  type CareReviewFormat,
  downloadCareRunReview,
} from "~/lib/traeger-care-review-api";
import { traegerPath } from "~/lib/traeger-url";
import { invoiceCountLabel } from "./care-billing-format";
import { formatEuro } from "./contribution-format";
import { DEBIT_GAP_LABELS } from "./sepa-format";

const logger = createLogger({ component: "CareRunReview" });

const REVIEW_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "xlsx",
  "csv",
]);

function childName(invoice: CareInvoice): string {
  return `${invoice.childFirstName} ${invoice.childLastName}`.trim();
}

/** The row menu of a preview invoice: leave it out of the release. */
export function ExclusionMenu({
  invoice,
  onExclude,
  onFamilyBilling,
}: Readonly<{
  invoice: CareInvoice;
  onExclude: (invoice: CareInvoice) => void;
  onFamilyBilling?: (invoice: CareInvoice) => void;
}>) {
  return (
    <OverflowMenu
      ariaLabel={`Aktionen für die Rechnung von ${childName(invoice)}`}
      triggerSize="sm"
      items={[
        {
          label: "Ausschließen",
          description: "Bekommt bei der Freigabe keine Nummer",
          onClick: () => onExclude(invoice),
        },
        ...(onFamilyBilling
          ? [
              {
                label: "Empfänger und Zahlung",
                description: "Für alle Rechnungen dieser Familie",
                onClick: () => onFamilyBilling(invoice),
              },
            ]
          : []),
      ]}
    />
  );
}

/** „Prüfliste“: the preview as Excel or CSV, without invoice numbers. */
export function CareReviewExport({ runId }: Readonly<{ runId: string }>) {
  const [busy, setBusy] = useState<string | null>(null);
  const errors = useApiErrorDisplay();
  const download = async (format: CareReviewFormat) => {
    setBusy(format);
    try {
      const file = await downloadCareRunReview(runId, format);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_review_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "die Prüfliste" });
    } finally {
      setBusy(null);
    }
  };
  return (
    <ExportMenu
      label="Prüfliste"
      ariaLabel="Prüfliste herunterladen"
      options={REVIEW_FORMATS}
      busyKey={busy}
      onSelect={(option) => {
        if (option.format === "xlsx" || option.format === "csv") {
          void download(option.format);
        }
      }}
    />
  );
}

/**
 * The invoices left out of the release, each with its reason and the way
 * back into the run.
 */
export function CareExcludedCard({
  invoices,
  returning,
  onInclude,
}: Readonly<{
  invoices: readonly CareInvoice[];
  /** The contract whose invoice is on its way back. */
  returning: string | null;
  onInclude: (invoice: CareInvoice) => void;
}>) {
  if (invoices.length === 0) return null;
  const total = invoices.reduce((sum, invoice) => sum + invoice.totalCents, 0);
  return (
    <SectionCard
      title="Ausgeschlossen"
      titleBadge={
        <span className="text-sm font-normal text-gray-600 tabular-nums">
          {invoiceCountLabel(invoices.length)} · {formatEuro(total)}
        </span>
      }
      description="Diese Rechnungen bekommen bei der Freigabe keine Nummer. Das Kind kommt im nächsten Lauf dieses Monats wieder."
    >
      <ResponsiveActionList>
        {invoices.map((invoice) => (
          <li
            key={`${invoice.contractId}-${invoice.service}`}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="font-semibold text-gray-900">
                  {childName(invoice)}
                </span>
                <span className="text-gray-600">{invoice.schoolName}</span>
                <span className="text-gray-500 tabular-nums">
                  {formatEuro(invoice.totalCents)}
                </span>
              </p>
              <p className="mt-0.5 text-sm wrap-anywhere text-gray-700">
                Grund: {invoice.exclusion?.reason}
              </p>
              {invoice.exclusion ? (
                <p className="mt-0.5 text-xs text-gray-500">
                  Ausgeschlossen am {formatDate(invoice.exclusion.excludedAt)}
                </p>
              ) : null}
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <Button
                type="button"
                variant="surface"
                size="compact"
                isLoading={returning === invoice.contractId}
                loadingText="Wird aufgenommen …"
                disabled={returning !== null}
                aria-label={`Wieder aufnehmen: Rechnung von ${childName(invoice)}`}
                onClick={() => onInclude(invoice)}
              >
                Wieder aufnehmen
              </Button>
            </div>
          </li>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}

/** At most this many hints show before „weitere anzeigen“. */
const DEBIT_HINTS_SHOWN = 5;

interface DebitHint {
  readonly invoice: CareInvoice;
  readonly text: string;
}

/** The SEPA hints of a preview (#4695): direct debits without a valid
 * mandate or creditor identifier, and debits sooner than the
 * pre-notification days of the Träger. */
export function debitHints(invoices: readonly CareInvoice[]): DebitHint[] {
  const hints: DebitHint[] = [];
  for (const invoice of invoices) {
    // An excluded invoice is not issued; one of 0 € collects nothing.
    if (invoice.exclusion || invoice.totalCents <= 0) continue;
    const payment = invoice.payment;
    if (payment?.debitGap) {
      hints.push({
        invoice,
        text:
          payment.method === "transfer"
            ? `${DEBIT_GAP_LABELS[payment.debitGap]}. Geht als Überweisung, fällig am ${formatDate(payment.dueOn)}.`
            : `${DEBIT_GAP_LABELS[payment.debitGap]}. Die Freigabe ist angehalten.`,
      });
    } else if (payment?.mandate?.noticeShort) {
      hints.push({
        invoice,
        text: `Abbuchung schon am ${formatDate(payment.dueOn)}. Das ist früher als Ihre Vorabankündigung erlaubt.`,
      });
    }
  }
  return hints;
}

/**
 * The families that pay by direct debit but cannot be collected as planned,
 * one row each with the way to their mandate. A creditor identifier that is
 * missing applies to every row and stands once above the list.
 */
export function CareDebitHintsCard({
  runId,
  invoices,
}: Readonly<{ runId: string; invoices: readonly CareInvoice[] }>) {
  const [expanded, setExpanded] = useState(false);
  const hints = debitHints(invoices);
  if (hints.length === 0) return null;
  const creditorMissing = hints.some(
    (hint) => hint.invoice.payment?.debitGap === "creditor_missing",
  );
  const shown = expanded ? hints : hints.slice(0, DEBIT_HINTS_SHOWN);
  return (
    <SectionCard
      title="Lastschrift"
      titleBadge={
        <span className="text-sm font-normal text-gray-600 tabular-nums">
          {hints.length === 1 ? "1 Familie" : `${hints.length} Familien`}
        </span>
      }
      description={
        creditorMissing
          ? "Ohne Gläubiger-ID buchen Sie bei keiner Familie ab. Tragen Sie sie unter Einstellungen → Abrechnung ein."
          : "Bei diesen Familien klappt die Abbuchung so nicht. Prüfen Sie das Mandat vor der Freigabe."
      }
      actions={
        creditorMissing ? (
          <ButtonLink
            href={traegerPath("/traeger/einstellungen/abrechnung")}
            variant="outline"
            size="md"
          >
            Gläubiger-ID eintragen
          </ButtonLink>
        ) : null
      }
    >
      <ResponsiveActionList>
        {shown.map(({ invoice, text }) => (
          <li
            key={`${invoice.contractId}-${invoice.service}`}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="font-semibold text-gray-900">
                  {childName(invoice)}
                </span>
                <span className="text-gray-600">{invoice.schoolName}</span>
                <span className="text-gray-500 tabular-nums">
                  {formatEuro(invoice.totalCents)}
                </span>
              </p>
              <p className="mt-0.5 text-sm text-gray-700">{text}</p>
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <ButtonLink
                href={traegerPath(
                  `/traeger/abrechnung/monatsbeitraege/${runId}/empfaenger/${invoice.contractId}`,
                )}
                variant="surface"
                size="compact"
                aria-label={`Mandat prüfen: Familie von ${childName(invoice)}`}
              >
                Mandat prüfen
              </ButtonLink>
            </div>
          </li>
        ))}
      </ResponsiveActionList>
      {hints.length > DEBIT_HINTS_SHOWN ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          className="mt-3"
          aria-expanded={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded
            ? "Weniger anzeigen"
            : `${hints.length - DEBIT_HINTS_SHOWN} weitere anzeigen`}
        </Button>
      ) : null}
    </SectionCard>
  );
}
