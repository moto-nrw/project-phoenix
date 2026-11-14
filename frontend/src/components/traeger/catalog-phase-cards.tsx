"use client";

// Die Karten einer Anmeldephase des Trägers (#4704): das letzte Ergebnis je
// Schule und die Betreuungsangebote der Phase.

import { useRouter } from "next/navigation";
import { useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { schoolsConcept } from "~/lib/concepts/schools";
import { formatBerlinDate } from "~/lib/date-helpers";
import { errorCodeMessage } from "~/lib/error-presentation";
import { createLogger } from "~/lib/logger";
import type { TraegerSchool } from "~/lib/traeger-api";
import {
  type TraegerOffering,
  type TraegerPhaseDetail,
  withdrawTraegerOffering,
} from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";
import {
  type Badge,
  type SchoolStanding,
  outcomeBadge,
  schoolStandings,
  weekdayList,
} from "./catalog-format";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "CatalogPhaseCards" });

/** The reason a school gave, in the words of the error catalog. */
export function refusalText(errorCode: string | null): string | null {
  return errorCode ? errorCodeMessage(errorCode, "die Anmeldephase") : null;
}

function schoolName(
  schools: readonly TraegerSchool[],
  schoolId: string,
): string {
  return (
    schools.find((school) => school.id === schoolId)?.name ??
    "Schule gehört nicht mehr zum Träger"
  );
}

function standingDetail(standing: SchoolStanding): string {
  const { row } = standing;
  if (standing.leaving) {
    return "Nicht mehr ausgewählt. Beim nächsten Übertragen wird die Kopie zurückgezogen.";
  }
  if (row?.outcome === "rejected") {
    return refusalText(row.errorCode) ?? "Die Schule hat abgelehnt.";
  }
  if (row?.outcome === "stale") {
    return "Die Schule hat seit der Vorschau etwas geändert. Bitte übertragen Sie noch einmal.";
  }
  if (row?.outcome === "applied" && row.recordedAt) {
    return `Am ${formatBerlinDate(row.recordedAt)} übernommen`;
  }
  return "–";
}

export function CatalogSchoolsCard({
  phase,
  schools,
}: Readonly<{
  phase: TraegerPhaseDetail;
  schools: readonly TraegerSchool[];
}>) {
  const standings = schoolStandings(
    phase,
    schools.map((school) => school.id),
  );
  const columns: DataTableColumn<SchoolStanding>[] = [
    {
      key: "school",
      header: "Schule",
      render: (standing) => (
        <span className="font-medium text-gray-900">
          {schoolName(schools, standing.schoolId)}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "outcome",
      header: "Letztes Übertragen",
      render: (standing) => (
        <StatusBadge {...outcomeBadge(standing.row?.outcome ?? null)} />
      ),
      stacked: "meta",
    },
    {
      key: "detail",
      header: "Hinweis",
      render: (standing) => (
        <span className="text-sm text-gray-700">
          {standingDetail(standing)}
        </span>
      ),
      stacked: "field",
    },
  ];
  return (
    <SectionCard
      concept={schoolsConcept}
      title="Schulen"
      description="Jede Schule darf ihre Kopie ändern und ausschalten. Ihre Änderungen bleiben auch beim nächsten Übertragen."
    >
      <DataTable
        columns={columns}
        rows={standings}
        getRowKey={(standing) => standing.schoolId}
        stackedOnMobile
        emptyState={
          <EmptyState
            variant="compact"
            title="Keine Schule ausgewählt."
            description="Wählen Sie unten bei „Schulen“ aus, wer die Anmeldephase bekommt."
          />
        }
      />
    </SectionCard>
  );
}

/** The state of one offering of the phase. */
function offeringStatus(offering: TraegerOffering): Badge {
  if (offering.withdrawnAt) return { label: "Zurückgezogen", tone: "gray" };
  if (offering.released === null) {
    return { label: "Noch nicht übertragen", tone: "warning" };
  }
  if (JSON.stringify(offering.draft) !== JSON.stringify(offering.released)) {
    return { label: "Geändert, nicht übertragen", tone: "warning" };
  }
  return { label: "Übertragen", tone: "green" };
}

function priceSummary(offering: TraegerOffering): string {
  const { prices } = offering.draft;
  if (prices.length === 0) return "Keine Preise";
  const amounts = prices.map((price) => price.monthlyCents);
  const low = Math.min(...amounts);
  const high = Math.max(...amounts);
  const scale = prices.some((price) => price.daysPerWeek !== null);
  const range =
    low === high
      ? formatEuro(low)
      : `${formatEuro(low)} bis ${formatEuro(high)}`;
  return `${range} ${scale ? "nach Tagen je Woche" : "je Wochentag"}`;
}

export function CatalogOfferingsCard({
  phase,
  editable,
  onWithdrawn,
}: Readonly<{
  phase: TraegerPhaseDetail;
  /** A withdrawn phase keeps its offerings as they are. */
  editable: boolean;
  onWithdrawn: (offering: TraegerOffering) => void;
}>) {
  const router = useRouter();
  const [target, setTarget] = useState<TraegerOffering | null>(null);
  const [withdrawing, setWithdrawing] = useState(false);
  const { show } = useApiErrorDisplay();
  const { success } = useToast();
  const base = `/traeger/anmeldephasen/${phase.id}/angebote`;

  const confirmWithdraw = async () => {
    if (!target) return;
    setWithdrawing(true);
    try {
      const withdrawn = await withdrawTraegerOffering(target.id);
      onWithdrawn(withdrawn);
      setTarget(null);
      success(
        `„${withdrawn.draft.fields.name}“ ist zurückgezogen. Übertragen Sie jetzt, damit es aus den Schulen verschwindet.`,
      );
    } catch (err) {
      logger.error("traeger_catalog_offering_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void show(err, { object: "das Angebot" });
    } finally {
      setWithdrawing(false);
    }
  };

  const columns: DataTableColumn<TraegerOffering>[] = [
    {
      key: "name",
      header: "Angebot",
      render: (offering) => (
        <span className="font-medium text-gray-900">
          {offering.draft.fields.name}
        </span>
      ),
      sortValue: (offering) => offering.draft.fields.sortOrder,
      stacked: "title",
    },
    {
      key: "days",
      header: "Tage",
      render: (offering) => weekdayList(offering.draft.fields.availableDays),
      stacked: "field",
    },
    {
      key: "prices",
      header: "Monatspreis",
      render: (offering) => priceSummary(offering),
      stacked: "field",
    },
    {
      key: "status",
      header: "Stand",
      render: (offering) => <StatusBadge {...offeringStatus(offering)} />,
      stacked: "field",
    },
    {
      key: "actions",
      header: "",
      stacked: "meta",
      render: (offering) =>
        editable && !offering.withdrawnAt ? (
          <OverflowMenu
            ariaLabel={`Aktionen für ${offering.draft.fields.name}`}
            items={[
              {
                label: "Bearbeiten",
                description: "Öffnet das Angebot zum Ändern",
                href: traegerPath(`${base}/${offering.id}`),
                onClick: () => undefined,
              },
              {
                label: "Zurückziehen",
                description:
                  "Nimmt das Angebot aus der Anmeldephase. moto fragt vorher nach.",
                destructive: true,
                onClick: () => setTarget(offering),
              },
            ]}
          />
        ) : null,
    },
  ];

  return (
    <SectionCard
      title="Betreuungsangebote"
      description="Preise und Vertragsvorlage gehen beim Übertragen an jede Schule dieser Anmeldephase."
      actions={
        editable ? (
          <ButtonLink
            href={traegerPath(`${base}/neu`)}
            variant="outline"
            size="md"
            tooltip="Öffnet ein neues Betreuungsangebot für diese Anmeldephase."
          >
            Angebot anlegen
          </ButtonLink>
        ) : null
      }
    >
      <DataTable
        columns={columns}
        rows={[...phase.offerings].sort(
          (a, b) => a.draft.fields.sortOrder - b.draft.fields.sortOrder,
        )}
        getRowKey={(offering) => offering.id}
        stackedOnMobile
        rowHasInteractiveControls
        onRowClick={
          editable
            ? (offering) => {
                if (!offering.withdrawnAt) {
                  router.push(traegerPath(`${base}/${offering.id}`));
                }
              }
            : undefined
        }
        emptyState={
          <EmptyState
            variant="compact"
            title="Noch kein Angebot."
            description="Legen Sie oben ein Angebot an, zum Beispiel die Frühbetreuung."
          />
        }
      />
      <ConfirmationModal
        isOpen={target !== null}
        onClose={() => setTarget(null)}
        onConfirm={() => void confirmWithdraw()}
        title="Angebot zurückziehen?"
        confirmText="Zurückziehen"
        confirmVariant="danger"
        isConfirmLoading={withdrawing}
        loadingText="Wird zurückgezogen…"
      >
        <p className="text-sm text-gray-700">
          „{target?.draft.fields.name}“ verschwindet aus den Schulen, sobald Sie
          das nächste Mal übertragen. Hat ein Kind das Angebot schon gebucht,
          schaltet moto es dort nur aus. Das lässt sich nicht rückgängig machen.
        </p>
      </ConfirmationModal>
    </SectionCard>
  );
}
