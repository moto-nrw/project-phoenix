"use client";

// Rechnungen und Beträge je Familie (#4691): je Familie die Zahl der
// Rechnungen, wer zahlt und die Summen ausgestellt, storniert,
// gutgeschrieben und die offene Forderung. Eine Familie sind alle Schreiben
// an dieselbe E-Mail-Adresse. moto gleicht keine Zahlungseingänge ab
// (#6348): offen ist, was die Rechnungen nach Storno und Gutschrift noch
// verlangen, nie ein geprüfter Zahlungsrückstand. Nur die Rolle
// mit dem Recht „Abrechnung“ sieht die Seite. Ein Klick öffnet die Familie.
// Seit #6692 stehen hier auch Familien, die nur zur Ferienbetreuung
// angemeldet sind, schon vor der ersten Rechnung. Jede Familie hat ihre
// Debitorennummer; das Büro ändert sie mit dem Stift in ihrer Zelle. Die
// Nummer prüft das Format des Trägers (#6675).

import { Pencil, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  BillingAmountCards,
  sumAmounts,
} from "~/components/traeger/billing-amount-cards";
import {
  PAYMENT_TONES,
  billingTabs,
  familyMatches,
  familyName,
  familySortName,
  paymentLabel,
} from "~/components/traeger/billing-family-format";
import { paymentMethodsText } from "~/components/traeger/billing-payment-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { DEBTOR_NUMBER_TEXT_CLASS } from "~/components/traeger/family-debtor-number";
import { FamilyDebtorNumberModal } from "~/components/traeger/family-debtor-number-modal";
import { useDebtorNumberFormat } from "~/components/traeger/use-debtor-number-format";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import NavigationLink from "~/components/ui/navigation-link";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { Button } from "~/components/ui/button";
import { useApiLoadError } from "~/contexts/ToastContext";
import { parentsConcept } from "~/lib/concepts/parents";
import { createLogger } from "~/lib/logger";
import {
  type BillingFamily,
  fetchBillingFamilies,
} from "~/lib/traeger-billing-families-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerBillingFamiliesPage" });

type OpenFilter = "" | "offen" | "ausgeglichen";

const OPEN_FILTER_LABELS: Record<Exclude<OpenFilter, "">, string> = {
  offen: "Mit offener Forderung",
  ausgeglichen: "Ohne offene Forderung",
};

type NumberFilter = "" | "ohne" | "mit";

const NUMBER_FILTER_LABELS: Record<Exclude<NumberFilter, "">, string> = {
  ohne: "Ohne Debitorennummer",
  mit: "Mit Debitorennummer",
};

function euroCell(cents: number) {
  return cents === 0 ? (
    <span className="text-gray-400 tabular-nums">{formatEuro(0)}</span>
  ) : (
    <span className="tabular-nums">{formatEuro(cents)}</span>
  );
}

/** The page of a family with documents. */
function familyHref(family: BillingFamily): string {
  return traegerPath(
    `/traeger/abrechnung/familien/${encodeURIComponent(family.id)}`,
  );
}

/** The numbers the family's invoices copied besides the family's own. */
function otherInvoiceNumbers(family: BillingFamily): string[] {
  return family.debtorNumbers.filter(
    (number) => number !== family.familyDebtorNumber,
  );
}

/**
 * The family's own number with its change button in place (#6692); numbers
 * that only older invoices carry stand below it. The button sits at the
 * same place in every row, whatever the length of the number; a family
 * without a number shows „–“ and „Eintragen“. The button keeps the
 * row click from opening the family.
 */
function DebtorCell({
  family,
  onChange,
}: Readonly<{
  family: BillingFamily;
  onChange: (family: BillingFamily) => void;
}>) {
  const others = otherInvoiceNumbers(family);
  const name = familyName(family) || family.familyEmail;
  const number = family.familyDebtorNumber;
  return (
    <span className="block w-full max-w-52 min-w-0">
      <span className="flex min-w-0 items-center justify-between gap-3">
        {number ? (
          <span
            className={`block min-w-0 text-sm break-all ${DEBTOR_NUMBER_TEXT_CLASS}`}
          >
            {number}
          </span>
        ) : (
          <span className="block text-sm text-gray-400">–</span>
        )}
        <span data-row-control className="shrink-0">
          {number ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`Debitorennummer von ${name} ändern`}
              tooltip="Debitorennummer ändern"
              onClick={(event) => {
                event.stopPropagation();
                onChange(family);
              }}
            >
              <Pencil aria-hidden="true" className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              aria-label={`Debitorennummer für ${name} eintragen`}
              onClick={(event) => {
                event.stopPropagation();
                onChange(family);
              }}
            >
              <Plus aria-hidden="true" className="h-4 w-4" />
              Eintragen
            </Button>
          )}
        </span>
      </span>
      {others.length > 0 ? (
        // On a phone the line wraps below the number instead of widening
        // the cell, so the pencil stays next to the number in every row.
        <span className="block text-xs wrap-anywhere text-gray-500 max-sm:w-0 max-sm:min-w-full">
          Auf Rechnungen:{" "}
          <span className="font-medium text-gray-700 tabular-nums">
            {others.join(", ")}
          </span>
        </span>
      ) : null}
    </span>
  );
}

function familyColumns(
  onChangeNumber: (family: BillingFamily) => void,
): DataTableColumn<BillingFamily>[] {
  return [
    {
      key: "family",
      header: "Familie",
      sortValue: familySortName,
      render: (family) => (
        <span className="flex min-w-0 items-center gap-3">
          <MotoConceptIcon
            concept={parentsConcept}
            size={18}
            className="hidden shrink-0 sm:block"
          />
          <span className="min-w-0">
            <span className="flex min-w-0 items-center gap-2">
              {family.hasDocuments ? (
                <NavigationLink
                  href={familyHref(family)}
                  className="truncate font-medium text-gray-900 hover:underline"
                  onClick={(event) => event.stopPropagation()}
                >
                  {familyName(family) || family.familyEmail}
                </NavigationLink>
              ) : (
                <span className="truncate font-medium text-gray-900">
                  {familyName(family) || family.familyEmail}
                </span>
              )}
              {family.holidayOnly ? (
                <span className="shrink-0">
                  <StatusBadge label="Nur Ferien" tone="outline" />
                </span>
              ) : null}
            </span>
            <span className="block truncate text-sm text-gray-500">
              {family.recipientEmail}
            </span>
          </span>
        </span>
      ),
      stacked: "title",
    },
    {
      key: "debtor",
      header: "Debitorennummer",
      sortValue: (family) => family.familyDebtorNumber || null,
      render: (family) => (
        <DebtorCell family={family} onChange={onChangeNumber} />
      ),
    },
    {
      key: "payment",
      header: "Wer zahlt",
      render: (family) =>
        family.payments.length > 0 ? (
          <span className="flex max-w-52 min-w-0 flex-wrap gap-1.5">
            {family.payments.map((payment) => (
              <StatusBadge
                key={`${payment.kind}-${payment.costBearerName}`}
                label={paymentLabel(payment)}
                tone={PAYMENT_TONES[payment.kind]}
                truncate
              />
            ))}
          </span>
        ) : (
          "–"
        ),
    },
    {
      key: "method",
      header: "Zahlungsweg",
      render: (family) => paymentMethodsText(family.paymentMethods) ?? "–",
    },
    {
      key: "issued",
      header: "Ausgestellt",
      align: "right",
      sortValue: (family) => family.issuedCents,
      render: (family) => euroCell(family.issuedCents),
      stacked: "hidden",
    },
    {
      key: "cancelled",
      header: "Storniert",
      align: "right",
      sortValue: (family) => family.cancelledCents,
      render: (family) => euroCell(family.cancelledCents),
      stacked: "hidden",
    },
    {
      key: "credited",
      header: "Gutgeschrieben",
      align: "right",
      sortValue: (family) => family.creditedCents,
      render: (family) => euroCell(family.creditedCents),
      stacked: "hidden",
    },
    {
      key: "open",
      header: "Offene Forderung",
      align: "right",
      sortValue: (family) => family.openCents,
      render: (family) =>
        family.openCents > 0 ? (
          <span className="font-semibold text-gray-900 tabular-nums">
            {formatEuro(family.openCents)}
          </span>
        ) : (
          <span className="text-gray-400 tabular-nums">{formatEuro(0)}</span>
        ),
      stacked: "meta",
    },
  ];
}

function matchesNumberFilter(family: BillingFamily, filter: NumberFilter) {
  if (filter === "ohne") return !family.familyDebtorNumber;
  if (filter === "mit") return Boolean(family.familyDebtorNumber);
  return true;
}

function matchesOpenFilter(family: BillingFamily, filter: OpenFilter) {
  if (filter === "offen") return family.openCents > 0;
  if (filter === "ausgeglichen") return family.openCents === 0;
  return true;
}

export default function TraegerBillingFamiliesPage() {
  const router = useRouter();
  const [families, setFamilies] = useState<BillingFamily[] | null>(null);
  const [search, setSearch] = useState("");
  const [openFilter, setOpenFilter] = useState<OpenFilter>("");
  const [numberFilter, setNumberFilter] = useState<NumberFilter>("");
  const [changing, setChanging] = useState<BillingFamily | null>(null);
  const format = useDebtorNumberFormat();
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setFamilies(await fetchBillingFamilies());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_billing_families_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Familien",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const open = useCallback(
    (family: BillingFamily) => router.push(familyHref(family)),
    [router],
  );
  const columns = useMemo(() => familyColumns(setChanging), []);
  const shown =
    families?.filter(
      (family) =>
        familyMatches(family, search) &&
        matchesOpenFilter(family, openFilter) &&
        matchesNumberFilter(family, numberFilter),
    ) ?? [];
  const withOpen = families?.filter((family) => family.openCents > 0).length;
  const withoutNumber = families?.filter(
    (family) => !family.familyDebtorNumber,
  ).length;
  const filtered =
    search.trim() !== "" || openFilter !== "" || numberFilter !== "";

  const filters: FilterConfig[] = [
    {
      id: "open",
      label: "Offene Forderung",
      type: "dropdown",
      value: openFilter,
      onChange: (value) => setOpenFilter(value as OpenFilter),
      options: [
        { value: "", label: "Alle Familien" },
        { value: "offen", label: OPEN_FILTER_LABELS.offen },
        { value: "ausgeglichen", label: OPEN_FILTER_LABELS.ausgeglichen },
      ],
    },
    {
      id: "number",
      label: "Debitorennummer",
      type: "dropdown",
      value: numberFilter,
      onChange: (value) => setNumberFilter(value as NumberFilter),
      options: [
        { value: "", label: "Alle Nummern" },
        { value: "ohne", label: NUMBER_FILTER_LABELS.ohne },
        { value: "mit", label: NUMBER_FILTER_LABELS.mit },
      ],
    },
  ];
  const activeFilters: ActiveFilter[] = [
    ...(openFilter
      ? [
          {
            id: "open",
            label: OPEN_FILTER_LABELS[openFilter],
            onRemove: () => setOpenFilter(""),
          },
        ]
      : []),
    ...(numberFilter
      ? [
          {
            id: "number",
            label: NUMBER_FILTER_LABELS[numberFilter],
            onRemove: () => setNumberFilter(""),
          },
        ]
      : []),
  ];
  const resetFilters = () => {
    setSearch("");
    setOpenFilter("");
    setNumberFilter("");
  };

  return (
    <TenantPage
      title="Abrechnung"
      info="Hier sehen Sie je Familie alle Rechnungen, Stornos und Gutschriften und ihre Debitorennummer. Eine Familie ist eine E-Mail-Adresse. Familien aus der Ferienbetreuung stehen hier schon nach dem Bestätigen der Anmeldung. Die offene Forderung ist, was die Rechnungen nach Storno und Gutschrift noch verlangen. Zahlungen sieht moto nicht. Ob eine Familie schon gezahlt hat, prüfen Sie in Ihrer Buchhaltung."
      tabs={billingTabs("familien")}
      stats={
        families && families.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: families.length,
                label: families.length === 1 ? "Familie" : "Familien",
              },
              { value: withOpen ?? 0, label: "mit offener Forderung" },
              {
                value: withoutNumber ?? 0,
                label: "ohne Debitorennummer",
                // Ein Klick zeigt genau diese Familien (#6675).
                ...(withoutNumber && numberFilter !== "ohne"
                  ? {
                      onSelect: () => setNumberFilter("ohne"),
                      selectLabel: "Familien ohne Debitorennummer zeigen",
                    }
                  : {}),
              },
            ]}
          />
        ) : null
      }
      statsLoading={!families && !pageLoad.error}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Name, E-Mail oder Debitorennummer",
      }}
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setOpenFilter("");
        setNumberFilter("");
      }}
      error={pageLoad.error}
      loading={!pageLoad.error && !families}
      overlays={
        <FamilyDebtorNumberModal
          family={changing}
          format={format}
          onClose={() => setChanging(null)}
          onSaved={load}
        />
      }
    >
      {families ? (
        <>
          <BillingAmountCards amounts={sumAmounts(shown)} />
          <DataTable
            columns={columns}
            rows={shown}
            getRowKey={(family) => family.familyEmail || family.id}
            defaultSortKey="family"
            stackedOnMobile
            stackedLayout="row"
            stackedDetailKey="debtor"
            rowHasInteractiveControls
            onRowClick={open}
            canClickRow={(family) => family.hasDocuments}
            emptyState={
              filtered ? (
                <EmptyState
                  title="Keine Familie passt zur Suche."
                  description="Ändern Sie die Suche oder den Filter."
                  action={
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      onClick={resetFilters}
                    >
                      Filter zurücksetzen
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={<MotoConceptIcon concept={parentsConcept} size={48} />}
                  title="Noch keine Familie."
                  description="Eine Familie steht hier, sobald Sie eine Ferienanmeldung bestätigen oder eine Rechnung freigeben."
                />
              )
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
