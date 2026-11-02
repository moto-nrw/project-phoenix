"use client";

import { useSession } from "next-auth/react";
import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { HolidayHealthCell } from "~/components/holiday/holiday-health-lines";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatDate } from "~/lib/date-helpers";
import {
  downloadHolidayParticipants,
  fetchHolidayParticipants,
  fetchHostedHolidayWeeks,
  formatHolidayHostAnswer,
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
import { HolidayGuestGroupCard } from "./holiday-guest-group-card";

const logger = createLogger({ component: "FerienwochenPage" });

// No SSE event covers holiday bookings, so the page reconciles on focus and
// once a minute: cancellations and rebookings show up, and a week drops out
// after its last day without a reload.
const REFRESH = { revalidateOnFocus: true, refreshInterval: 60_000 };

// Ferienwochen (#4665): die ausrichtende OGS sieht vor und während ihrer
// Woche die Kinder, deren Anmeldung der Träger bestätigt hat, mit Zahlen je
// Zeitmodell und Heimatschule. Nach dem letzten Tag der Woche ist die Liste
// weg. Die Seite liest nur; Anmeldungen prüft und bestätigt der Träger.
//
// Die Leitung wählt hier auch die Feriengruppe der Woche (#4666): Die
// bestätigten Feriengäste kommen dann in diese Gruppe und in den Check-in.
//
// Gibt der Träger Fragen ohne Gesundheitsangabe frei, steht je Frage eine
// Spalte in der Liste, dazu die zweite sorgeberechtigte Person (#5202).
// Die Gesundheitsangaben der Kinder, deren Eltern der Weitergabe zugestimmt
// haben, stehen in einer Spalte „Gesundheit“; Export und Druck tragen sie
// nicht (#4722).
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

/** Die zweite sorgeberechtigte Person mit anrufbarer Nummer. */
function secondGuardianCell(child: HolidayParticipant) {
  const guardian = child.secondGuardian;
  if (!guardian) return "–";
  if (!guardian.phone) return guardian.name;
  return (
    <span className="flex flex-col">
      <span>{guardian.name}</span>
      <a
        className="underline"
        href={`tel:${guardian.phone.replaceAll(" ", "")}`}
      >
        {guardian.phone}
      </a>
    </span>
  );
}

const participantColumns = (
  list: HolidayParticipantList,
): DataTableColumn<HolidayParticipant>[] => {
  const names = new Map(list.timeModels.map((model) => [model.id, model.name]));
  const withSecondGuardian = list.participants.some(
    (child) => child.secondGuardian !== null,
  );
  const withHealth = list.participants.some((child) => child.health !== null);
  const answer = (child: HolidayParticipant, key: string) =>
    child.answers.find((entry) => entry.key === key);
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
    ...(withSecondGuardian
      ? [
          {
            key: "secondGuardian",
            header: "Zweite sorgeberechtigte Person",
            render: secondGuardianCell,
            sortValue: (child: HolidayParticipant) =>
              child.secondGuardian?.name ?? null,
          },
        ]
      : []),
    ...list.questions.map((question) => ({
      key: `answer:${question.key}`,
      header: question.label,
      render: (child: HolidayParticipant) =>
        formatHolidayHostAnswer(answer(child, question.key)),
      sortValue: (child: HolidayParticipant) => {
        const value = answer(child, question.key)?.value ?? null;
        return typeof value === "boolean" ? String(value) : value;
      },
    })),
    ...(withHealth
      ? [
          {
            key: "health",
            header: "Gesundheit",
            render: (child: HolidayParticipant) => (
              <HolidayHealthCell health={child.health} />
            ),
          },
        ]
      : []),
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
  const { data: session } = useSession();
  const canAssignGuests = hasPermission(session, "users:create");
  // Tenant-scoped keys: the list holds children's names and birth dates, so
  // another school's cache entry must never show here.
  const weeks = useSWRAuth(
    isReady ? "holiday-weeks" : null,
    fetchHostedHolidayWeeks,
    REFRESH,
  );
  const [chosenWeek, setChosenWeek] = useState<string | null>(null);
  const selectedWeek =
    weeks.data?.find((week) => week.id === chosenWeek) ?? weeks.data?.[0];
  const participants = useSWRAuth(
    selectedWeek ? `holiday-participants:${selectedWeek.id}` : null,
    () => fetchHolidayParticipants(selectedWeek?.id ?? ""),
    // A week switch must not show the previous week's children under the new
    // week's heading while its list loads.
    { ...REFRESH, keepPreviousData: false },
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
      info="Hier stehen die Ferienwochen, die Ihre OGS für den Träger ausrichtet. Sie sehen nur Kinder, deren Anmeldung der Träger bestätigt hat. Nach dem letzten Tag der Woche ist die Liste nicht mehr da. Gesundheitsangaben stehen nur hier, nicht in der Excel- oder CSV-Datei."
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
      {list && selectedWeek && (
        <div className="flex flex-col gap-6">
          <HolidayGuestGroupCard
            key={selectedWeek.id}
            week={selectedWeek}
            canAssign={canAssignGuests}
            onAssigned={() => void weeks.mutate()}
          />
          <ParticipantList list={list} />
        </div>
      )}
    </TenantPage>
  );
}

function ParticipantList({ list }: { readonly list: HolidayParticipantList }) {
  const columns = useMemo(() => participantColumns(list), [list]);
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
        description={[
          "Nur zur Information. Anmeldungen prüft und bestätigt der Träger.",
          list.questions.length > 0
            ? "Dazu kommen Antworten der Eltern, die der Träger für Sie freigibt."
            : "",
          list.participants.some((child) => child.health !== null)
            ? "Gesundheitsangaben stehen bei den Kindern, deren Eltern zugestimmt haben."
            : "",
        ]
          .filter(Boolean)
          .join(" ")}
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
