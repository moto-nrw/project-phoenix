"use client";

// Kostenträger-Anteile der Verträge (#4687): welcher Kostenträger (Bildung
// und Teilhabe, Jugendamt, sonstiger Zuschuss) wie viel vom Monatsbeitrag
// eines Kindes übernimmt, bis wann, und auf welchen Bescheid es sich stützt.
// Nur das Recht „Abrechnung“ sieht die Seite; jedes Lesen steht im
// Zugriffsprotokoll des Vertrags. Die Stellen selbst pflegt die Abrechnung
// unter Einstellungen → Abrechnung.

import { HandCoins } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  COST_SHARE_KIND_LABELS,
  costShareAmountText,
} from "~/components/traeger/care-cost-share-format";
import {
  CareCostShareModal,
  type CostShareTarget,
} from "~/components/traeger/care-cost-share-modal";
import { billingTabs } from "~/components/traeger/billing-family-format";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareCostShare,
  type CareCostShareContract,
  type CareCostShareSet,
  deleteCareCostShare,
  fetchCareCostShares,
} from "~/lib/traeger-care-cost-shares-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerKostentraegerPage" });

type Show = "all" | "with" | "expiring";

const SHOW_OPTIONS: { value: Show; label: string }[] = [
  { value: "all", label: "Alle Kinder" },
  { value: "with", label: "Mit Anteil" },
  { value: "expiring", label: "Läuft bald ab" },
];

/** "01.08.2026 bis 31.07.2027". */
function sharePeriod(share: CareCostShare): string {
  return `${formatDate(share.validFrom)} bis ${formatDate(share.validUntil)}`;
}

function ShareStatus({ share }: Readonly<{ share: CareCostShare }>) {
  switch (share.status) {
    case "expiring":
      return (
        <StatusBadge
          tone="orange"
          label={`Endet am ${formatDate(share.validUntil)}`}
        />
      );
    case "upcoming":
      return (
        <StatusBadge tone="blue" label={`Ab ${formatDate(share.validFrom)}`} />
      );
    case "expired":
      return <StatusBadge tone="gray" label="Abgelaufen" />;
    default:
      return <StatusBadge tone="green" label="Gilt" />;
  }
}

function ShareCell({ share }: Readonly<{ share: CareCostShare }>) {
  return (
    <div className="min-w-0">
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-gray-900 max-sm:justify-end">
        <span className="font-medium wrap-anywhere">
          {share.costBearerName}
        </span>
        <span className="text-gray-600">
          {COST_SHARE_KIND_LABELS[share.kind]}
        </span>
        <span className="font-semibold tabular-nums">
          {costShareAmountText(share)}
        </span>
        <ShareStatus share={share} />
      </p>
      <p className="mt-0.5 text-xs wrap-anywhere text-gray-600">
        {sharePeriod(share)} · {share.reference} · geprüft am{" "}
        {formatDate(share.checkedOn)}
      </p>
    </div>
  );
}

export default function TraegerKostentraegerPage() {
  const [data, setData] = useState<CareCostShareSet | null>(null);
  const [search, setSearch] = useState("");
  const [show, setShow] = useState<Show>("all");
  const [target, setTarget] = useState<CostShareTarget | null>(null);
  const [deleting, setDeleting] = useState<{
    contract: CareCostShareContract;
    share: CareCostShare;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const deleteErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setData(await fetchCareCostShares());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_cost_shares_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Kostenträger-Anteile",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      setData(await deleteCareCostShare(deleting.share.id));
      success("Der Anteil ist gelöscht.");
      setDeleting(null);
    } catch (err) {
      logger.error("care_cost_share_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "den Anteil" });
    } finally {
      setBusy(false);
    }
  };

  const contracts = data?.contracts ?? [];
  const withShare = contracts.filter((contract) =>
    contract.shares.some((share) => share.status !== "expired"),
  ).length;
  const activeBearers = (data?.costBearers ?? []).filter(
    (bearer) => bearer.active,
  ).length;

  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return (data?.contracts ?? []).filter((contract) => {
      if (show === "with" && contract.shares.length === 0) return false;
      if (
        show === "expiring" &&
        !contract.shares.some((share) => share.status === "expiring")
      ) {
        return false;
      }
      return needle
        ? `${contract.childFirstName} ${contract.childLastName} ${contract.schoolName} ${contract.shares.map((share) => share.costBearerName).join(" ")}`
            .toLowerCase()
            .includes(needle)
        : true;
    });
  }, [data, search, show]);

  const filters: FilterConfig[] = [
    {
      id: "show",
      label: "Anzeigen",
      type: "buttons",
      value: show,
      onChange: (value) => setShow(value as Show),
      options: SHOW_OPTIONS,
    },
  ];

  const columns: DataTableColumn<CareCostShareContract>[] = [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      className: "w-56 align-top",
      render: (contract) => (
        <div className="min-w-0">
          <p className="font-semibold text-gray-900">
            {contract.childFirstName} {contract.childLastName}
          </p>
          <p className="mt-0.5 truncate text-sm text-gray-600">
            {contract.schoolName}
          </p>
        </div>
      ),
    },
    {
      key: "shares",
      header: "Anteile",
      stacked: "field",
      className: "align-top",
      render: (contract) =>
        contract.shares.length === 0 ? (
          <span className="text-sm text-gray-500">
            Kein Anteil, die Familie zahlt alles
          </span>
        ) : (
          <div className="space-y-3">
            {contract.shares.map((share) => (
              <ShareCell key={share.id} share={share} />
            ))}
          </div>
        ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      className: "w-12 align-top",
      render: (contract) => (
        <OverflowMenu
          ariaLabel={`Anteile: ${contract.childFirstName} ${contract.childLastName}`}
          items={[
            ...(activeBearers > 0
              ? [
                  {
                    label: "Anteil eintragen",
                    onClick: () => setTarget({ contract, share: null }),
                  },
                ]
              : []),
            ...contract.shares.flatMap((share) => {
              const description = `${share.costBearerName}, ${sharePeriod(share)}`;
              return [
                {
                  label: "Anteil ändern",
                  description,
                  onClick: () => setTarget({ contract, share }),
                },
                {
                  label: "Anteil löschen",
                  description,
                  destructive: true,
                  onClick: () => setDeleting({ contract, share }),
                },
              ];
            }),
          ]}
        />
      ),
    },
  ];

  return (
    <TenantPage
      title="Abrechnung"
      info={`Hier tragen Sie ein, welcher Kostenträger einen Teil des Monatsbeitrags eines Kindes bezahlt, zum Beispiel Bildung und Teilhabe oder das Jugendamt. Die Familie bekommt dann nur den Rest in Rechnung. moto speichert nur Art, Anteil, Gültigkeit und das Aktenzeichen, keinen Bescheid. Jedes Öffnen dieser Seite steht im Zugriffsprotokoll. Die Stellen selbst legen Sie unter Einstellungen → Abrechnung an.`}
      stats={
        data && contracts.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: withShare,
                label:
                  withShare === 1 ? "Kind mit Anteil" : "Kinder mit Anteil",
              },
              {
                value: data.expiring,
                label: data.expiring === 1 ? "endet bald" : "enden bald",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Kind, Schule oder Kostenträger",
      }}
      filters={filters}
      tabs={billingTabs("kostentraeger")}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <>
            <CareCostShareModal
              data={data}
              target={target}
              onClose={() => setTarget(null)}
              onSaved={(next) => {
                setData(next);
                setTarget(null);
              }}
            />
            <ConfirmDeleteModal
              isOpen={deleting !== null}
              onClose={() => {
                setDeleting(null);
                deleteErrors.clear();
              }}
              onConfirm={() => void remove()}
              title="Anteil löschen?"
              description={
                deleting
                  ? `${deleting.share.costBearerName} übernimmt dann für ${deleting.contract.childFirstName} ${deleting.contract.childLastName} nichts mehr (${sharePeriod(deleting.share)}). Neue Monatsläufe stellen der Familie den ganzen Beitrag in Rechnung. Ausgestellte Rechnungen bleiben, wie sie sind.`
                  : ""
              }
              confirmLabel="Löschen"
              loadingLabel="Wird gelöscht …"
              gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
              loading={busy}
              error={deleteErrors.error}
            />
          </>
        ) : null
      }
    >
      {data && contracts.length === 0 ? (
        <EmptyState
          icon={<HandCoins className="h-12 w-12" aria-hidden="true" />}
          title="Noch kein Vertrag."
          description="Anteile tragen Sie für Kinder mit Betreuungsvertrag ein."
        />
      ) : null}
      {data && contracts.length > 0 && activeBearers === 0 ? (
        <Alert
          type="info"
          message="Legen Sie zuerst die Kostenträger an, zum Beispiel das Jugendamt oder die Stelle für Bildung und Teilhabe."
          action={
            <ButtonLink
              href={traegerPath("/traeger/einstellungen/abrechnung")}
              variant="surface"
              size="compact"
            >
              Kostenträger anlegen
            </ButtonLink>
          }
        />
      ) : null}
      {data && data.expiring > 0 ? (
        <Alert
          type="warning"
          message={
            data.expiring === 1
              ? `Ein Bescheid endet in den nächsten ${data.warningDays} Tagen. Fragen Sie den neuen Bescheid an und tragen Sie den Anteil danach neu ein. Sonst zahlt die Familie ab dem nächsten Monat den ganzen Beitrag.`
              : `${data.expiring} Bescheide enden in den nächsten ${data.warningDays} Tagen. Fragen Sie die neuen Bescheide an und tragen Sie die Anteile danach neu ein. Sonst zahlen die Familien ab dem nächsten Monat den ganzen Beitrag.`
          }
          action={
            show === "expiring" ? undefined : (
              <Button
                type="button"
                variant="surface"
                size="compact"
                onClick={() => {
                  setShow("expiring");
                }}
              >
                Nur diese zeigen
              </Button>
            )
          }
        />
      ) : null}
      {data && contracts.length > 0 ? (
        <DataTable
          columns={columns}
          rows={rows}
          getRowKey={(contract) => contract.contractId}
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Kein Kind passt zur Auswahl."
              description="Ändern Sie die Suche oder zeigen Sie alle Kinder."
              action={
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  onClick={() => {
                    setSearch("");
                    setShow("all");
                  }}
                >
                  Alle Kinder zeigen
                </Button>
              }
            />
          }
        />
      ) : null}
    </TenantPage>
  );
}
