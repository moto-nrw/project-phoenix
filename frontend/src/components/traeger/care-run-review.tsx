"use client";

// Die Prüfphase eines Monatslaufs vor der Freigabe (#5916, SJR-Abgleich E1
// A): das Menü „Ausschließen“ je Rechnung, die Karte der ausgeschlossenen
// Rechnungen mit dem Weg zurück und die Prüfliste als Excel oder CSV. Eine
// ausgeschlossene Rechnung bekommt bei der Freigabe keine Nummer; ihr Kind
// kommt im nächsten Lauf desselben Monats wieder. Die Laufseite lädt diese
// Datei erst in der Prüfphase (care-run-review-lazy.tsx), die Dialoge erst,
// wenn jemand sie öffnet.

import { Ban, Pencil, Wallet } from "lucide-react";
import { useState } from "react";
import { Button, ButtonLink } from "~/components/ui/button";
import {
  type ExportMenuOption,
  ExportMenu,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import {
  ResponsiveActionList,
  ResponsiveActionRow,
} from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import type { BillingDocument } from "~/lib/traeger-billing-api";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import {
  type CareReviewFormat,
  downloadCareRunReview,
} from "~/lib/traeger-care-review-api";
import { traegerPath } from "~/lib/traeger-url";
import { invoiceCountLabel } from "./care-billing-format";
import { invoiceKindLabel } from "./care-review-format";
import {
  type DebitHint,
  type DebitItem,
  careDebitItems,
  debitHints,
  holidayDebitItems,
} from "./care-debit-hints";
export { careDebitItems, debitHints, holidayDebitItems };
import { formatEuro } from "./contribution-format";
import { familiesLabel } from "./release-open-points";

const logger = createLogger({ component: "CareRunReview" });

const REVIEW_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "xlsx",
  "csv",
]);

function childName(invoice: CareInvoice): string {
  return `${invoice.childFirstName} ${invoice.childLastName}`.trim();
}

/** Names one invoice of a run: its contract and what it bills (#6346). */
function invoiceKey(invoice: CareInvoice): string {
  return `${invoice.contractId}-${invoice.service}`;
}

/** The row menu of a preview invoice: leave it out of the release. */
export function ExclusionMenu({
  invoice,
  onExclude,
  onFamilyBilling,
  onAmend,
}: Readonly<{
  invoice: CareInvoice;
  onExclude: (invoice: CareInvoice) => void;
  onFamilyBilling?: (invoice: CareInvoice) => void;
  /** Opens the page that changes this invoice of the run (#6197). */
  onAmend?: (invoice: CareInvoice) => void;
}>) {
  return (
    <OverflowMenu
      ariaLabel={`Aktionen für die Rechnung von ${childName(invoice)}`}
      items={[
        ...(onAmend
          ? [
              {
                label: "Rechnung ändern",
                icon: <Pencil className="size-4" aria-hidden />,
                description:
                  "Nur diese Rechnung: Betrag, Positionen, Empfänger, Anschrift",
                onClick: () => onAmend(invoice),
              },
            ]
          : []),
        {
          label: "Ausschließen",
          icon: <Ban className="size-4" aria-hidden />,
          description: "Nur diese Rechnung. Sie bekommt keine Nummer.",
          onClick: () => onExclude(invoice),
        },
        ...(onFamilyBilling
          ? [
              {
                label: "Empfänger und Zahlung",
                icon: <Wallet className="size-4" aria-hidden />,
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
      label="Prüfliste herunterladen"
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
 * The invoices left out of the release, each with what it bills, its reason
 * and the way back into the run. Each row is one invoice (#6346).
 */
export function CareExcludedCard({
  invoices,
  returning,
  onInclude,
}: Readonly<{
  invoices: readonly CareInvoice[];
  /** The invoice on its way back, as `contractId-service`. */
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
      description="Diese Rechnungen bekommen bei der Freigabe keine Nummer. Sie kommen im nächsten Lauf dieses Monats wieder."
    >
      <ResponsiveActionList>
        {invoices.map((invoice) => (
          <ResponsiveActionRow key={invoiceKey(invoice)}>
            <div className="min-w-0">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="font-semibold text-gray-900">
                  {childName(invoice)}
                </span>
                <span className="font-medium text-gray-700">
                  {invoiceKindLabel(invoice)}
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
                <p className="mt-0.5 text-sm text-gray-500">
                  Ausgeschlossen am {formatDate(invoice.exclusion.excludedAt)}
                </p>
              ) : null}
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <Button
                type="button"
                variant="surface"
                size="compact"
                isLoading={returning === invoiceKey(invoice)}
                loadingText="Wird aufgenommen …"
                disabled={returning !== null}
                aria-label={`Wieder aufnehmen: ${invoiceKindLabel(invoice)} von ${childName(invoice)}`}
                onClick={() => onInclude(invoice)}
              >
                Wieder aufnehmen
              </Button>
            </div>
          </ResponsiveActionRow>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}

/** At most this many hints show before „weitere anzeigen“. */
const DEBIT_HINTS_SHOWN = 5;

/** The hints card of a holiday run preview. */
export function HolidayDebitHintsCard({
  documents,
  dueDatesHref,
  earliest,
}: Readonly<{
  documents: readonly BillingDocument[];
  /** The run's due-date page, for a debit that comes too early. */
  dueDatesHref?: string;
  earliest?: string | null;
}>) {
  return (
    <DebitHintsCard
      items={holidayDebitItems(documents)}
      dueDatesHref={dueDatesHref}
      earliest={earliest}
    />
  );
}

/** The hints card of a care run preview. */
export function CareDebitHintsCard({
  invoices,
  dueDatesHref,
  earliest,
}: Readonly<{
  invoices: readonly CareInvoice[];
  /** The run's due-date page, for a debit that comes too early. */
  dueDatesHref?: string;
  /** The first day the pre-notification allows, when the run knows it. */
  earliest?: string | null;
}>) {
  return (
    <DebitHintsCard
      items={careDebitItems(invoices)}
      dueDatesHref={dueDatesHref}
      earliest={earliest}
    />
  );
}

function DebitHintsCard({
  items,
  dueDatesHref,
  earliest = null,
}: Readonly<{
  items: readonly DebitItem[];
  dueDatesHref?: string;
  /** The first day the pre-notification allows, when the run knows it. */
  earliest?: string | null;
}>) {
  const [expanded, setExpanded] = useState(false);
  const hints = debitHints(items);
  if (hints.length === 0) return null;
  const creditorMissing = hints.some(
    (hint) => hint.item.payment?.debitGap === "creditor_missing",
  );
  // A missing mandate is entered, any other gap is checked.
  const mandateAction = (item: DebitItem) =>
    item.payment?.debitGap === "mandate_missing"
      ? "Mandat erfassen"
      : "Mandat prüfen";
  // Open rows first; a family that pays by transfer is information only.
  const isTransfer = (hint: DebitHint) =>
    !hint.dueTooSoon && hint.item.payment?.method === "transfer";
  const ordered = [
    ...hints.filter((hint) => !isTransfer(hint)),
    ...hints.filter(isTransfer),
  ];
  const shown = expanded ? ordered : ordered.slice(0, DEBIT_HINTS_SHOWN);
  // A debit that comes too early is fixed once for the whole run, by a later
  // date: the card says it once and carries the one way there. A family
  // that falls back to a transfer needs nothing before the release: it is
  // named, but not counted as open.
  const tooSoon = hints.filter((hint) => hint.dueTooSoon).length;
  const transfers = hints.filter(
    (hint) => !hint.dueTooSoon && hint.item.payment?.method === "transfer",
  ).length;
  const halted = hints.length - tooSoon - transfers;
  const open = tooSoon + halted;
  const subject = (count: number) =>
    count === hints.length
      ? "Bei diesen Familien"
      : `Bei ${familiesLabel(count)}`;
  const description = creditorMissing
    ? "Ohne Gläubiger-ID buchen Sie bei keiner Familie ab. Tragen Sie sie unter Einstellungen → Abrechnungsvorgaben ein."
    : [
        // Short: each row names its own date and its earliest day.
        tooSoon > 0
          ? `${subject(tooSoon)} ist die Abbuchung zu früh für Ihre Vorabankündigung. Legen Sie eine spätere Fälligkeit fest.`
          : null,
        halted > 0
          ? `${subject(halted)} fehlt ein gültiges Mandat. Klären Sie das vor der Freigabe.`
          : null,
        // A transfer needs nothing before the release: beside open points
        // its rows under „Zur Information“ say it already.
        transfers > 0 && open === 0
          ? transfers === hints.length
            ? "Diese Familien zahlen per Überweisung, weil ein gültiges Mandat fehlt."
            : `${familiesLabel(transfers)} ${transfers === 1 ? "zahlt" : "zahlen"} per Überweisung, weil ein gültiges Mandat fehlt.`
          : null,
      ]
        .filter(Boolean)
        .join(" ");
  return (
    <SectionCard
      title="Lastschrift"
      titleBadge={
        open > 0 ? (
          <span className="text-moto-orange-strong text-sm font-medium tabular-nums">
            {`${familiesLabel(open)} offen`}
          </span>
        ) : (
          <span className="text-sm font-normal text-gray-600 tabular-nums">
            {familiesLabel(hints.length)}
          </span>
        )
      }
      description={description}
      actions={
        creditorMissing ? (
          <ButtonLink
            href={traegerPath("/traeger/einstellungen/abrechnung")}
            variant="outline"
            size="md"
          >
            Gläubiger-ID eintragen
          </ButtonLink>
        ) : tooSoon > 0 && dueDatesHref ? (
          <ButtonLink
            href={traegerPath(dueDatesHref)}
            variant="outline"
            size="md"
          >
            Fälligkeiten ändern
          </ButtonLink>
        ) : null
      }
    >
      {(() => {
        // Open rows first; a family that pays by transfer pays all the
        // same, so it stands apart as information.
        const renderRow = ({ item, text, dueTooSoon }: DebitHint) => (
          <ResponsiveActionRow key={item.key}>
            <div className="min-w-0">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <span className="font-semibold text-gray-900">{item.name}</span>
                {item.detail ? (
                  <span className="text-gray-600">{item.detail}</span>
                ) : null}
                <span className="text-gray-500 tabular-nums">
                  {formatEuro(item.totalCents)}
                </span>
              </p>
              <p className="mt-0.5 text-sm text-gray-700">
                {dueTooSoon && dueDatesHref && item.payment
                  ? `Abbuchung am ${formatDate(item.payment.dueOn)}: zu früh${earliest ? `, frühestens am ${formatDate(earliest)}` : ""}.`
                  : text}
              </p>
            </div>
            {/* The mandate stays one click away also when only the date is
                too early: the central „Fälligkeiten ändern“ fixes the date,
                this link checks the family's mandate. */}
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <ButtonLink
                href={traegerPath(item.href)}
                variant="surface"
                size="compact"
                aria-label={`${mandateAction(item)}: ${item.family}`}
              >
                {mandateAction(item)}
              </ButtonLink>
            </div>
          </ResponsiveActionRow>
        );
        const openRows = shown.filter((hint) => !isTransfer(hint));
        const infoRows = shown.filter(isTransfer);
        return (
          <>
            {openRows.length > 0 ? (
              <ResponsiveActionList>
                {openRows.map(renderRow)}
              </ResponsiveActionList>
            ) : null}
            {infoRows.length > 0 ? (
              <>
                {openRows.length > 0 ? (
                  <SectionLabel as="h3" className="mt-5 mb-2">
                    Zur Information
                  </SectionLabel>
                ) : null}
                <ResponsiveActionList>
                  {infoRows.map(renderRow)}
                </ResponsiveActionList>
              </>
            ) : null}
          </>
        );
      })()}
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
