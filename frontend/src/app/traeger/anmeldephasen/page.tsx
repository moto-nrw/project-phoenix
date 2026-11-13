"use client";

// Anmeldephasen des Trägers (#4704): eine Anmeldephase mit Angeboten für
// mehrere Schulen. Von hier aus anlegen und öffnen.

import { ClipboardList, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import {
  phaseStatusBadge,
  serviceRange,
  targetSchoolIds,
} from "~/components/traeger/catalog-format";
import { useTraegerCatalogPhases } from "~/components/traeger/use-traeger-catalog";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import type { TraegerPhase } from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";

/** „Alle Schulen“ or the number of chosen schools. */
function schoolsLabel(phase: TraegerPhase): string {
  if (phase.allSchools) return "Alle Schulen";
  const count = phase.schools.filter((school) => school.selected).length;
  return count === 1 ? "1 Schule" : `${count} Schulen`;
}

/** How many target schools refused or missed the last rollout. */
function openSchools(
  phase: TraegerPhase,
  allSchoolIds: readonly string[],
): number {
  if (phase.withdrawnAt || phase.releasedVersion === null) return 0;
  const rows = new Map(phase.schools.map((row) => [row.schoolId, row]));
  return targetSchoolIds(phase, allSchoolIds).filter((schoolId) => {
    const outcome = rows.get(schoolId)?.outcome ?? null;
    return outcome === "rejected" || outcome === "stale";
  }).length;
}

export default function TraegerCatalogPhasesPage() {
  const router = useRouter();
  const { data, isLoading, loadError } = useTraegerCatalogPhases();
  const { schools } = useTraegerSchools();
  const phases = data ?? [];
  const allSchoolIds = (schools ?? []).map((school) => school.id);

  const columns: DataTableColumn<TraegerPhase>[] = [
    {
      key: "name",
      header: "Anmeldephase",
      render: (phase) => (
        <span className="font-medium text-gray-900">{phase.draft.name}</span>
      ),
      sortValue: (phase) => phase.draft.name,
      stacked: "title",
    },
    {
      key: "service",
      header: "Leistungszeitraum",
      render: (phase) =>
        serviceRange(phase.draft.serviceStartDate, phase.draft.serviceEndDate),
      sortValue: (phase) => phase.draft.serviceStartDate,
      stacked: "meta",
    },
    {
      key: "schools",
      header: "Schulen",
      render: (phase) => {
        const open = openSchools(phase, allSchoolIds);
        return (
          <span className="flex flex-wrap items-center gap-2">
            {schoolsLabel(phase)}
            {open > 0 ? (
              <StatusBadge
                label={
                  open === 1 ? "1 nicht übernommen" : `${open} nicht übernommen`
                }
                tone="warning"
              />
            ) : null}
          </span>
        );
      },
      stacked: "meta",
    },
    {
      key: "status",
      header: "Stand",
      render: (phase) => <StatusBadge {...phaseStatusBadge(phase)} />,
      sortValue: (phase) => phaseStatusBadge(phase).label,
      stacked: "meta",
    },
  ];

  return (
    <TenantPage
      title="Anmeldephasen"
      info="Hier legen Sie eine Anmeldephase mit Betreuungsangeboten einmal für mehrere Schulen an. Jede Schule bekommt eine eigene Kopie, sobald Sie sie freigeben und übertragen. Die Schule darf ihre Kopie ändern oder ausschalten."
      stats={
        phases.length > 0 ? (
          <span>
            {phases.length === 1
              ? "1 Anmeldephase"
              : `${phases.length} Anmeldephasen`}
          </span>
        ) : null
      }
      statsLoading={isLoading}
      error={loadError}
      actions={
        <ButtonLink
          href={traegerPath("/traeger/anmeldephasen/neu")}
          variant="primary"
          size="md"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          Anmeldephase anlegen
        </ButtonLink>
      }
    >
      <DataTable
        columns={columns}
        rows={phases}
        getRowKey={(phase) => phase.id}
        isLoading={isLoading}
        loadingRowCount={3}
        stackedOnMobile
        stackedLayout="row"
        onRowClick={(phase) =>
          router.push(traegerPath(`/traeger/anmeldephasen/${phase.id}`))
        }
        emptyState={
          <EmptyState
            icon={<ClipboardList className="h-12 w-12" aria-hidden="true" />}
            title="Noch keine Anmeldephase."
            description="Legen Sie oben eine Anmeldephase an. Danach fügen Sie die Betreuungsangebote hinzu."
          />
        }
      />
    </TenantPage>
  );
}
