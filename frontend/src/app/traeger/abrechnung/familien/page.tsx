"use client";

// Rechnungen und Beträge je Familie (#4691): je Familie die Zahl der
// Rechnungen, wer zahlt und die Summen ausgestellt, storniert,
// gutgeschrieben und die offene Forderung. Eine Familie sind alle Schreiben
// an dieselbe E-Mail-Adresse. moto gleicht keine Zahlungseingänge ab
// (#6348): offen ist, was die Rechnungen nach Storno und Gutschrift noch
// verlangen, nie ein geprüfter Zahlungsrückstand. Nur die Rolle
// mit dem Recht „Abrechnung“ sieht die Seite. Ein Klick öffnet die Familie.

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
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
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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

function euroCell(cents: number) {
  return cents === 0 ? (
    <span className="text-gray-400 tabular-nums">{formatEuro(0)}</span>
  ) : (
    <span className="tabular-nums">{formatEuro(cents)}</span>
  );
}

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
    sortValue: (family) => family.debtorNumbers[0] ?? null,
    render: (family) =>
      family.debtorNumbers.length > 0 ? (
        <span className="font-mono text-sm">
          {family.debtorNumbers.join(", ")}
        </span>
      ) : (
        "–"
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
        object: "die Familien",
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

  return (
    <TenantPage
      title="Abrechnung"
      info="Hier sehen Sie je Familie alle Rechnungen, Stornos und Gutschriften. Eine Familie sind alle Schreiben an dieselbe E-Mail-Adresse. Die offene Forderung ist, was die Rechnungen nach Storno und Gutschrift noch verlangen. Zahlungen sieht moto nicht. Ob eine Familie schon gezahlt hat, prüfen Sie in Ihrer Buchhaltung."
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
      onClearAllFilters={() => setOpenFilter("")}
      error={pageLoad.error}
      loading={!pageLoad.error && !families}
    >
      {families ? (
        <>
          <BillingAmountCards amounts={sumAmounts(shown)} />
          <DataTable
            columns={COLUMNS}
            rows={shown}
            getRowKey={(family) => family.id}
            defaultSortKey="family"
            stackedOnMobile
            stackedLayout="row"
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
                  description="Ändern Sie die Suche oder den Filter."
                  action={
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      tooltip="Leert Suche und Filter und zeigt wieder alle Familien."
                      onClick={resetFilters}
                    >
                      Filter zurücksetzen
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
