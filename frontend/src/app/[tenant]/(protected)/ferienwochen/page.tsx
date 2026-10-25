"use client";

import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import {
  downloadHolidayParticipants,
  fetchHolidayParticipants,
  fetchHostedHolidayWeeks,
  type HolidayHomeSchoolCount,
  type HolidayParticipant,
  type HolidayParticipantList,
  type HolidayTimeModelCount,
  type HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "FerienwochenPage" });

// Ferienwochen (#4665): die ausrichtende OGS sieht vor und während ihrer
// Woche die Kinder, deren Anmeldung der Träger bestätigt hat, mit Zahlen je
// Zeitmodell und Heimatschule. Nach dem letzten Tag der Woche ist die Liste
// weg. Die Seite liest nur; Anmeldungen prüft und bestätigt der Träger.
//
// Bauart 3 (Werkzeug): eine Liste zum Vorbereiten und Ausdrucken, kein
// Objekt, das hier bearbeitet wird.

/** „06.07. bis 10.07.2099“ */
function weekLabel(week: HostedHolidayWeek): string {
  return `${formatDate(week.startsOn).slice(0, 6)} bis ${formatDate(week.endsOn)}`;
}

function childCount(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

const participantColumns = (
  timeModels: HolidayTimeModelCount[],
): DataTableColumn<HolidayParticipant>[] => {
  const names = new Map(timeModels.map((model) => [model.id, model.name]));
  return [
    {
      key: "name",
      header: "Name",
      stacked: "title",
      className: "font-medium text-gray-800",
      render: (child) => `${child.lastName}, ${child.firstName}`,
      sortValue: (child) => `${child.lastName} ${child.firstName}`,
    },
    {
      key: "birthDate",
      header: "Geburtsdatum",
      render: (child) => formatDate(child.birthDate),
      sortValue: (child) => child.birthDate,
    },
    {
      key: "homeSchool",
      header: "Heimatschule",
      render: (child) => child.homeSchoolName,
      sortValue: (child) => child.homeSchoolName,
    },
    {
      key: "timeModel",
      header: "Zeitmodell",
      render: (child) => names.get(child.timeModelId) ?? "–",
      sortValue: (child) => names.get(child.timeModelId) ?? null,
    },
  ];
};

const timeModelColumns: DataTableColumn<HolidayTimeModelCount>[] = [
  {
    key: "name",
    header: "Zeitmodell",
    stacked: "title",
    render: (model) =>
      `${model.name} (${model.startsAt} bis ${model.endsAt} Uhr)`,
  },
  {
    key: "count",
    header: "Kinder",
    align: "right",
    stacked: "meta",
    render: (model) => model.count,
  },
];

const homeSchoolColumns: DataTableColumn<HolidayHomeSchoolCount>[] = [
  {
    key: "name",
    header: "Heimatschule",
    stacked: "title",
    render: (school) => school.name,
  },
  {
    key: "count",
    header: "Kinder",
    align: "right",
    stacked: "meta",
    render: (school) => school.count,
  },
];

export default function FerienwochenPage() {
  const { isReady, isLoading: permissionLoading } = useRequirePermission([
    "config:read",
  ]);
  // Tenant-scoped keys: the list holds children's names and birth dates, so
  // another school's cache entry must never show here.
  const weeks = useSWRAuth(
    isReady ? "holiday-weeks" : null,
    fetchHostedHolidayWeeks,
  );
  const [chosenWeek, setChosenWeek] = useState<string | null>(null);
  const selectedWeek =
    weeks.data?.find((week) => week.id === chosenWeek) ?? weeks.data?.[0];
  const participants = useSWRAuth(
    selectedWeek ? `holiday-participants:${selectedWeek.id}` : null,
    () => fetchHolidayParticipants(selectedWeek?.id ?? ""),
  );

  const weeksError = useSwrLoadError(weeks.error, "die Ferienwochen", () =>
    weeks.mutate(),
  );
  const participantsError = useSwrLoadError(
    participants.error,
    "die Teilnehmerliste",
    () => participants.mutate(),
  );

  const { show: showExportError } = useApiErrorDisplay();
  const retryExportRef = useRef<() => void>(() => undefined);
  const exportList = useCallback(
    async (format: "csv" | "xlsx") => {
      if (!selectedWeek) return;
      try {
        await downloadHolidayParticipants(selectedWeek.id, format);
      } catch (error) {
        logger.error("holiday_participants_export_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        retryExportRef.current = () => void exportList(format);
        void showExportError(error, {
          object: "die Teilnehmerliste",
          retry: () => retryExportRef.current(),
        });
      }
    },
    [selectedWeek, showExportError],
  );
  const exportRef = useRef(exportList);
  useLayoutEffect(() => {
    exportRef.current = exportList;
  });

  const list = participants.data;
  const loadingWeeks =
    permissionLoading || !isReady || (!weeks.data && !weeks.error);
  const loading =
    loadingWeeks ||
    (Boolean(selectedWeek) && !list && !participants.error) ||
    (Boolean(weeks.error) && !weeksError);

  const stats = useMemo(() => {
    if (!selectedWeek) return null;
    return `${selectedWeek.periodName} · ${weekLabel(selectedWeek)} · ${childCount(
      list?.participants.length ?? selectedWeek.participants,
    )}`;
  }, [selectedWeek, list]);

  return (
    <TenantPage
      title="Ferienwochen"
      info="Hier stehen die Ferienwochen, die Ihre OGS für den Träger ausrichtet. Sie sehen nur Kinder, deren Anmeldung der Träger bestätigt hat. Nach dem letzten Tag der Woche ist die Liste nicht mehr da."
      stats={stats}
      statsLoading={loadingWeeks}
      loading={loading}
      error={weeksError ?? participantsError}
      actions={
        selectedWeek ? (
          <OverflowMenu
            ariaLabel="Weitere Aktionen"
            items={[
              {
                label: "Als Excel herunterladen",
                onClick: () => void exportRef.current("xlsx"),
              },
              {
                label: "Als CSV herunterladen",
                onClick: () => void exportRef.current("csv"),
              },
            ]}
          />
        ) : undefined
      }
      tabs={
        weeks.data && weeks.data.length > 1
          ? {
              value: selectedWeek?.id ?? "",
              onChange: setChosenWeek,
              label: "Ferienwoche",
              items: weeks.data.map((week) => ({
                value: week.id,
                label: weekLabel(week),
              })),
            }
          : undefined
      }
      empty={
        weeks.data && weeks.data.length === 0
          ? {
              title: "Ihre OGS richtet gerade keine Ferienwoche aus",
              description: "Die Ferienwochen plant der Träger.",
            }
          : null
      }
    >
      {list && <ParticipantList list={list} />}
    </TenantPage>
  );
}

function ParticipantList({ list }: { readonly list: HolidayParticipantList }) {
  const columns = useMemo(
    () => participantColumns(list.timeModels),
    [list.timeModels],
  );
  return (
    <div className="flex flex-col gap-6">
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <SectionCard title="Je Zeitmodell">
          <DataTable
            columns={timeModelColumns}
            rows={list.timeModels}
            getRowKey={(model) => model.id}
          />
        </SectionCard>
        <SectionCard title="Je Heimatschule">
          <DataTable
            columns={homeSchoolColumns}
            rows={list.homeSchools}
            getRowKey={(school) => school.schoolId ?? "guests"}
            emptyState={
              <EmptyState variant="compact" title="Noch kein Kind bestätigt" />
            }
          />
        </SectionCard>
      </div>
      <SectionCard
        title="Kinder"
        description="Nur zur Information. Anmeldungen prüft und bestätigt der Träger."
      >
        <DataTable
          columns={columns}
          rows={list.participants}
          getRowKey={(child) => child.childId}
          defaultSortKey="name"
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Noch kein Kind bestätigt"
              description="Sobald der Träger eine Anmeldung bestätigt, steht das Kind hier."
            />
          }
        />
      </SectionCard>
    </div>
  );
}
