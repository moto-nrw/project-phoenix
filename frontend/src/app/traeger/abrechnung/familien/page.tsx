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
// Debitorennummer; das Büro ändert sie über das Menü in ihrer Zelle. Die
// Nummer prüft das Format des Trägers (#6675). Führt der Träger die Nummer
// je Kind (#7007), stehen in der Zelle die Kinder mit ihren Nummern, und
// Familien mit Vertrag stehen hier schon vor der ersten Rechnung.

import { Pencil, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import type { MouseEvent } from "react";
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
import { EmailText } from "~/components/traeger/email-text";
import { DEBTOR_NUMBER_TEXT_CLASS } from "~/components/traeger/family-debtor-number";
import {
  ChildDebtorNumbersModal,
  childLabel,
} from "~/components/traeger/child-debtor-numbers-modal";
import { FamilyDebtorNumberModal } from "~/components/traeger/family-debtor-number-modal";
import { useDebtorNumberSettings } from "~/components/traeger/use-debtor-number-format";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import NavigationLink from "~/components/ui/navigation-link";
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
  fetchFamilyChildren,
  withFamilyChildren,
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
    <span className="text-gray-500 tabular-nums">{formatEuro(0)}</span>
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

/** Whether the Träger keeps the number per child: the family has children. */
function perChild(family: BillingFamily): boolean {
  return family.children !== undefined;
}

/**
 * A family without a number: without its own, or with a child without one
 * when the Träger keeps the number per child (#7007).
 */
function lacksNumber(family: BillingFamily): boolean {
  if (perChild(family)) {
    return (family.children ?? []).some((child) => !child.debtorNumber);
  }
  return !family.familyDebtorNumber;
}

/** The numbers the family's invoices copied besides the family's own. */
function otherInvoiceNumbers(family: BillingFamily): string[] {
  return family.debtorNumbers.filter(
    (number) => number !== family.familyDebtorNumber,
  );
}

/**
 * The family's number and its row menu (#6692). Historical invoice numbers
 * stand below it; an absent family number shows „–“.
 */
function DebtorCell({
  family,
  onChange,
}: Readonly<{
  family: BillingFamily;
  onChange: (family: BillingFamily) => void;
}>) {
  if (perChild(family)) {
    return <ChildDebtorCell family={family} onChange={onChange} />;
  }
  const others = otherInvoiceNumbers(family);
  const name = familyName(family) || family.familyEmail;
  const number = family.familyDebtorNumber;
  return (
    <span className="block min-w-0 sm:w-40">
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
        <span
          data-row-control
          role="presentation"
          className="shrink-0"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <OverflowMenu
            ariaLabel={`Aktionen für ${name}`}
            items={[
              {
                label: number
                  ? "Debitorennummer ändern"
                  : "Debitorennummer eintragen",
                icon: number ? (
                  <Pencil aria-hidden="true" className="h-4 w-4" />
                ) : (
                  <Plus aria-hidden="true" className="h-4 w-4" />
                ),
                onClick: () => onChange(family),
              },
            ]}
          />
        </span>
      </span>
      {others.length > 0 ? (
        // Historical numbers wrap within the available width on phones.
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

/**
 * The children of the family with their own numbers (#7007) and the row
 * menu. A child without a number shows „–“.
 */
function ChildDebtorCell({
  family,
  onChange,
}: Readonly<{
  family: BillingFamily;
  onChange: (family: BillingFamily) => void;
}>) {
  const children = family.children ?? [];
  const name = familyName(family) || family.familyEmail;
  const complete = children.every((child) => child.debtorNumber);
  return (
    <span className="flex min-w-0 items-start justify-between gap-3 sm:w-56">
      {children.length > 0 ? (
        <span className="block min-w-0 space-y-0.5">
          {children.map((child) => (
            <span
              key={childLabel(child)}
              className="flex min-w-0 items-baseline gap-2 text-sm"
            >
              <span className="min-w-0 truncate text-gray-600">
                {child.firstName}
              </span>
              {child.debtorNumber ? (
                <span
                  className={`shrink-0 break-all ${DEBTOR_NUMBER_TEXT_CLASS}`}
                >
                  {child.debtorNumber}
                </span>
              ) : (
                <span className="shrink-0 text-gray-400">–</span>
              )}
            </span>
          ))}
        </span>
      ) : (
        <span className="block text-sm text-gray-500">
          Kein Kind mit Vertrag
        </span>
      )}
      {children.length > 0 ? (
        <span
          data-row-control
          role="presentation"
          className="shrink-0"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <OverflowMenu
            ariaLabel={`Aktionen für ${name}`}
            items={[
              {
                label: complete
                  ? "Nummern der Kinder ändern"
                  : "Nummern der Kinder eintragen",
                icon: complete ? (
                  <Pencil aria-hidden="true" className="h-4 w-4" />
                ) : (
                  <Plus aria-hidden="true" className="h-4 w-4" />
                ),
                onClick: () => onChange(family),
              },
            ]}
          />
        </span>
      ) : null}
    </span>
  );
}

function familyColumns(
  onChangeNumber: (family: BillingFamily) => void,
  childNumbers: boolean,
): DataTableColumn<BillingFamily>[] {
  const debtorHeader = childNumbers ? "Nummern der Kinder" : "Debitorennummer";
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
                  onClick={(event: MouseEvent<HTMLAnchorElement>) =>
                    event.stopPropagation()
                  }
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
            {/* The address gives way first: on a tablet all four sums stay
                in view beside it. There it wraps after its „@“ instead of
                being cut, so the whole address stays readable. */}
            <span className="block max-w-56 truncate text-sm text-gray-500 md:max-xl:max-w-36 md:max-xl:whitespace-normal">
              <EmailText email={family.recipientEmail} />
            </span>
          </span>
        </span>
      ),
      stacked: "title",
    },
    {
      key: "debtor",
      header: debtorHeader,
      stackedLabel: false,
      stackedRender: (family) => (
        <span className="block min-w-0">
          <span className="block text-xs wrap-anywhere text-gray-500">
            {debtorHeader}
          </span>
          <DebtorCell family={family} onChange={onChangeNumber} />
        </span>
      ),
      sortValue: (family) =>
        (perChild(family)
          ? family.children?.find((child) => child.debtorNumber)?.debtorNumber
          : family.familyDebtorNumber) || null,
      render: (family) => (
        <DebtorCell family={family} onChange={onChangeNumber} />
      ),
    },
    {
      key: "payment",
      header: "Zahlung",
      render: (family) => {
        const method = paymentMethodsText(family.paymentMethods);
        if (family.payments.length === 0 && !method) return "–";
        // Who pays and how, in one column so the amounts stay in view.
        return (
          <span className="block max-w-48 min-w-0 space-y-1">
            {family.payments.length > 0 ? (
              <span className="flex min-w-0 flex-wrap gap-1.5">
                {family.payments.map((payment) => (
                  <StatusBadge
                    key={`${payment.kind}-${payment.costBearerName}`}
                    label={paymentLabel(payment)}
                    tone={PAYMENT_TONES[payment.kind]}
                    truncate
                  />
                ))}
              </span>
            ) : null}
            {method ? (
              <span className="block text-sm text-gray-600">{method}</span>
            ) : null}
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
      header: "Offene Forderung",
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
}

function matchesNumberFilter(family: BillingFamily, filter: NumberFilter) {
  if (filter === "ohne") return lacksNumber(family);
  if (filter === "mit") return !lacksNumber(family);
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
  const settings = useDebtorNumberSettings();
  const format = settings?.format;
  const scope = settings?.scope;
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    if (!scope) return;
    try {
      // With a number per child the children come with a read of their own.
      const [listed, children] = await Promise.all([
        fetchBillingFamilies(),
        scope === "child" ? fetchFamilyChildren() : null,
      ]);
      setFamilies(children ? withFamilyChildren(listed, children) : listed);
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
  }, [clearLoadError, scope, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const open = useCallback(
    (family: BillingFamily) => router.push(familyHref(family)),
    [router],
  );
  const columns = useMemo(
    () => familyColumns(setChanging, scope === "child"),
    [scope],
  );
  const shown =
    families?.filter(
      (family) =>
        familyMatches(family, search) &&
        matchesOpenFilter(family, openFilter) &&
        matchesNumberFilter(family, numberFilter),
    ) ?? [];
  const withOpen = families?.filter((family) => family.openCents > 0).length;
  const withoutNumber = families?.filter(lacksNumber).length;
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
  const emptySearch = emptySearchText(
    search,
    activeFilters.map((filter) => filter.label),
  );

  return (
    <TenantPage
      title={billingAreaTitle("familien")}
      info={`Hier sehen Sie je Familie alle Rechnungen, Stornos und Gutschriften und ${scope === "child" ? "die Debitorennummer jedes Kindes" : "ihre Debitorennummer"}. Eine Familie ist eine E-Mail-Adresse. Familien aus der Ferienbetreuung stehen hier schon nach dem Bestätigen der Anmeldung. Die offene Forderung ist, was die Rechnungen nach Storno und Gutschrift noch verlangen. Zahlungen sieht moto nicht. Ob eine Familie schon gezahlt hat, prüfen Sie in Ihrer Buchhaltung.`}
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
                label:
                  scope === "child"
                    ? "mit Kind ohne Nummer"
                    : "ohne Debitorennummer",
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
        placeholder: "Name, E-Mail, Debitorennummer",
      }}
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setOpenFilter("");
        setNumberFilter("");
      }}
      error={pageLoad.error}
      // A list loads in the shape of its rows.
      loading={
        !pageLoad.error && !families ? (
          <ListSkeleton rows={6} avatar={false} />
        ) : (
          false
        )
      }
      overlays={
        <>
          <FamilyDebtorNumberModal
            family={changing && !perChild(changing) ? changing : null}
            format={format}
            onClose={() => setChanging(null)}
            onSaved={load}
          />
          <ChildDebtorNumbersModal
            family={changing && perChild(changing) ? changing : null}
            format={format}
            onClose={() => setChanging(null)}
            onSaved={load}
          />
        </>
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
            columns={columns}
            rows={shown}
            getRowKey={(family) => family.familyEmail || family.id}
            defaultSortKey="family"
            // Seven columns with four sums: narrower cells keep the open
            // claim in view beside the family on a laptop and a tablet.
            dense
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
