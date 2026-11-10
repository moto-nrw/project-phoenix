"use client";

// Rechnungen und Beträge je Familie (#4691): je Familie die Zahl der
// Rechnungen, wer zahlt und die Summen ausgestellt, storniert,
// gutgeschrieben und offen. Eine Familie sind alle Schreiben an dieselbe
// E-Mail-Adresse. moto gleicht keine Zahlungseingänge ab: offen ist, was
// die Rechnungen nach Storno und Gutschrift noch verlangen. Nur die Rolle
// mit dem Recht „Abrechnung“ sieht die Seite. Ein Klick öffnet die Familie.

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  BillingAmountCards,
  sumAmounts,
} from "~/components/traeger/billing-amount-cards";
import {
  billingAreaTitle,
  billingTabs,
  familyMatches,
  familyName,
  familySortName,
  PAYMENT_TONES,
  paymentLabel,
} from "~/components/traeger/billing-family-format";
import { paymentMethodsText } from "~/components/traeger/billing-payment-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { emptySearchText } from "~/components/traeger/empty-search";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { ListSkeleton } from "~/components/ui/page-skeletons";
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
  offen: "Mit berechnetem Betrag",
  ausgeglichen: "Nichts berechnet",
};

function euroCell(cents: number) {
  return cents === 0 ? (
    <span className="text-gray-500 tabular-nums">{formatEuro(0)}</span>
  ) : (
    <span className="tabular-nums">{formatEuro(cents)}</span>
  );
}

// Each sum has its own column, so families can be sorted and compared by
// every part of their invoices; who pays stands in its own column too.
const COLUMNS: DataTableColumn<BillingFamily>[] = [
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
          <span className="block font-medium text-gray-900">
            {familyName(family)}
          </span>
          {/* The address gives way first: on a tablet all four sums stay
              in view beside it. */}
          <span className="block max-w-56 truncate text-sm text-gray-500 md:max-xl:max-w-36">
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
    sortValue: (family) => family.debtorNumbers[0] ?? null,
    render: (family) =>
      family.debtorNumbers.length > 0 ? (
        <span className="font-mono text-sm whitespace-nowrap">
          {family.debtorNumbers.join(", ")}
        </span>
      ) : (
        "–"
      ),
  },
  {
    // Who pays and how in one column: all four sums keep their own
    // sortable column and still fit beside the family on a laptop.
    key: "payment",
    header: "Wer zahlt",
    render: (family) => {
      const method = paymentMethodsText(family.paymentMethods);
      return (
        <span className="block min-w-0">
          {family.payments.length > 0 ? (
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
          )}
          {method ? (
            <span className="mt-1 block text-sm text-gray-600">{method}</span>
          ) : null}
        </span>
      );
    },
    // On a phone one line under the name: the pills name who pays, the
    // payment method follows beside them; a family without both drops it.
    stackedLabel: false,
    stackedRender: (family) => {
      const method = paymentMethodsText(family.paymentMethods);
      if (family.payments.length === 0 && !method) return null;
      return (
        <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          {family.payments.map((payment) => (
            <StatusBadge
              key={`${payment.kind}-${payment.costBearerName}`}
              label={paymentLabel(payment)}
              tone={PAYMENT_TONES[payment.kind]}
              truncate
            />
          ))}
          {method ? <span className="text-gray-600">{method}</span> : null}
        </span>
      );
    },
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
    header: "Berechnet",
    align: "right",
    sortValue: (family) => family.openCents,
    render: (family) =>
      family.openCents > 0 ? (
        <span className="font-semibold text-gray-900 tabular-nums">
          {formatEuro(family.openCents)}
        </span>
      ) : (
        <span className="text-gray-500 tabular-nums">{formatEuro(0)}</span>
      ),
    stacked: "meta",
  },
];

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
        object: "die Liste der Familien",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const shown =
    families?.filter(
      (family) =>
        familyMatches(family, search) && matchesOpenFilter(family, openFilter),
    ) ?? [];
  const withOpen = families?.filter((family) => family.openCents > 0).length;
  const filtered = search.trim() !== "" || openFilter !== "";

  const filters: FilterConfig[] = [
    {
      id: "open",
      label: "Berechneter Betrag",
      type: "dropdown",
      value: openFilter,
      onChange: (value) => setOpenFilter(value as OpenFilter),
      options: [
        { value: "", label: "Alle Familien" },
        { value: "offen", label: OPEN_FILTER_LABELS.offen },
        { value: "ausgeglichen", label: OPEN_FILTER_LABELS.ausgeglichen },
      ],
    },
  ];
  const activeFilters: ActiveFilter[] = openFilter
    ? [
        {
          id: "open",
          label: OPEN_FILTER_LABELS[openFilter],
          onRemove: () => setOpenFilter(""),
        },
      ]
    : [];
  const resetFilters = () => {
    setSearch("");
    setOpenFilter("");
  };
  const emptySearch = emptySearchText(
    search,
    activeFilters.map((filter) => filter.label),
  );

  return (
    <TenantPage
      title={billingAreaTitle("familien")}
      info="Hier sehen Sie je Familie alle Rechnungen, Stornos und Gutschriften. Eine Familie sind alle Schreiben an dieselbe E-Mail-Adresse. Berechnet ist, was die Rechnungen nach Storno und Gutschrift verlangen. Ob eine Familie schon gezahlt hat, sieht moto nicht."
      tabs={billingTabs("familien")}
      stats={
        families && families.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: families.length,
                label: families.length === 1 ? "Familie" : "Familien",
              },
              { value: withOpen ?? 0, label: "mit berechnetem Betrag" },
            ]}
          />
        ) : null
      }
      statsLoading={!families && !pageLoad.error}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Name, E-Mail, Debitorennummer",
      }}
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => setOpenFilter("")}
      error={pageLoad.error}
      // A list loads in the shape of its rows.
      loading={
        !pageLoad.error && !families ? (
          <ListSkeleton rows={6} avatar={false} />
        ) : (
          false
        )
      }
    >
      {families ? (
        <>
          {/* Die Summen folgen Suche und Filter; die erste Kachel sagt, für
              wen. Ohne Treffer steht die Meldung direkt unter der Suche. */}
          {filtered && shown.length === 0 ? null : (
            <BillingAmountCards
              amounts={sumAmounts(shown)}
              scope={
                filtered
                  ? `Rechnungen der ${shown.length} gefundenen ${shown.length === 1 ? "Familie" : "Familien"}.`
                  : undefined
              }
            />
          )}
          <DataTable
            columns={COLUMNS}
            rows={shown}
            getRowKey={(family) => family.id}
            defaultSortKey="family"
            // Seven columns with four sums: narrower cells keep the charged
            // sum in view beside the family on a laptop and a tablet.
            dense
            stackedOnMobile
            stackedLayout="row"
            // On a phone who pays and how stands under the name.
            stackedDetailKey="payment"
            onRowClick={(family) =>
              router.push(
                traegerPath(
                  `/traeger/abrechnung/familien/${encodeURIComponent(family.id)}`,
                ),
              )
            }
            emptyState={
              filtered ? (
                <EmptyState
                  title="Keine Familie passt zur Suche."
                  description={emptySearch.description}
                  action={
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      onClick={resetFilters}
                    >
                      {emptySearch.actionLabel}
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={<MotoConceptIcon concept={parentsConcept} size={48} />}
                  title="Noch keine Rechnung."
                  description="Familien stehen hier, sobald ein Abrechnungslauf freigegeben ist."
                />
              )
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
