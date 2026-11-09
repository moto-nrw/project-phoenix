"use client";

// Schließtage und Feiertage des Trägers (#4702). Das Träger-Büro plant
// Schließtage wie Brückentage für einige oder alle Schulen und sieht die
// gesetzlichen Feiertage aus dem Bundesland jeder Schule. Das Bundesland
// legt das moto-Team fest; hier steht es nur zur Information.

import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useSession } from "next-auth/react";

import {
  calendarDayMarks,
  closingDaySchoolsLabel,
  dayCountLabel,
  formatClosingRange,
  formatDayWithWeekday,
  schoolsInState,
} from "~/components/traeger/closing-day-format";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { TraegerClosingDayModal } from "~/components/traeger/traeger-closing-day-modal";
import { TraegerYearCalendar } from "~/components/traeger/traeger-year-calendar";
import { Button } from "~/components/ui/button";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { calendarConcept } from "~/lib/concepts/calendar";
import { closingDaysConcept } from "~/lib/concepts/closingDays";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  deleteTraegerClosingDay,
  fetchTraegerCalendar,
  type TraegerClosingDay,
} from "~/lib/traeger-closing-days";

const logger = createLogger({ component: "TraegerClosingDaysPage" });

/** Das Recht, mit dem das Träger-Büro Schließtage plant. */
const SETTINGS_RIGHT = "traeger_settings:manage";

const INFO =
  "Hier planen Sie Schließtage für Ihre Schulen, zum Beispiel einen Brückentag. An einem Schließtag ist die OGS zu: keine Ferienbetreuung, keine Termine, kein Soll. Die Feiertage kommen aus dem Bundesland der Schule. Das Bundesland legt das moto-Team fest.";

export default function TraegerClosingDaysPage() {
  const { data: session } = useSession();
  const canManage = hasPermission(session ?? null, SETTINGS_RIGHT);
  const today = useBerlinToday();
  const thisYear = Number(today.slice(0, 4));
  const [year, setYear] = useState(thisYear);
  const from = `${year}-01-01`;
  const to = `${year}-12-31`;

  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.closingDays(from, to) : null,
    () => fetchTraegerCalendar(from, to),
    { keepPreviousData: true },
  );
  const { data, isLoading, mutate } = response;
  const swrError = useSwrLoadError(
    response.error,
    "die Schließtage und Feiertage",
    response.mutate,
  );
  const loadError = metadataLoadError(
    swrError,
    data !== undefined,
    response.error,
  );

  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<TraegerClosingDay | null>(null);
  const [deleting, setDeleting] = useState<TraegerClosingDay | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const deleteErrors = useApiFormError();
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const { success: toastSuccess } = useToast();

  const schools = useMemo(() => data?.schools ?? [], [data]);
  const closingDays = useMemo(() => data?.closingDays ?? [], [data]);
  const regions = useMemo(() => data?.regions ?? [], [data]);
  const marks = useMemo(
    () => (data ? calendarDayMarks(data) : new Map()),
    [data],
  );
  const holidayCount = useMemo(
    () => new Set(regions.flatMap((r) => r.holidays.map((h) => h.date))).size,
    [regions],
  );

  const beginCreate = () => {
    setEditing(null);
    setModalOpen(true);
  };
  const beginEdit = useCallback((day: TraegerClosingDay) => {
    setEditing(day);
    setModalOpen(true);
  }, []);

  const handleDelete = useCallback(async () => {
    if (!deleting) return;
    setDeleteLoading(true);
    deleteErrors.clear();
    try {
      await deleteTraegerClosingDay(deleting.id);
      toastSuccess(`Der Schließtag „${deleting.reason}“ ist gelöscht.`);
      setDeleting(null);
      void mutate();
    } catch (err) {
      logger.error("traeger_closing_day_delete_failed", {
        closing_day_id: deleting.id,
        error: err instanceof Error ? err.message : String(err),
      });
      await deleteErrors.show(err, {
        object: "das Löschen des Schließtags",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeleteLoading(false);
    }
  }, [deleting, deleteErrors, mutate, toastSuccess]);
  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDelete();
  });

  const columns = useMemo<DataTableColumn<TraegerClosingDay>[]>(
    () => [
      {
        key: "range",
        header: "Datum",
        sortValue: (day) => day.startDate,
        stacked: "title",
        render: (day) => (
          <div className="min-w-0">
            <p className="font-medium whitespace-nowrap text-gray-900 tabular-nums">
              {formatClosingRange(day)}
            </p>
            <p className="mt-0.5 text-xs text-gray-500">{dayCountLabel(day)}</p>
          </div>
        ),
      },
      {
        key: "reason",
        header: "Grund",
        sortValue: (day) => day.reason,
        stacked: "meta",
        render: (day) => (
          <span className="wrap-anywhere text-gray-900">{day.reason}</span>
        ),
      },
      {
        key: "schools",
        header: "Gilt für",
        stacked: "meta",
        render: (day) => {
          const label = closingDaySchoolsLabel(day, schools);
          return label === "Alle Schulen" ? (
            <StatusBadge label="Alle Schulen" tone="green" />
          ) : (
            <span className="text-sm wrap-anywhere text-gray-600">{label}</span>
          );
        },
      },
      ...(canManage
        ? [
            {
              key: "actions",
              header: "",
              align: "right" as const,
              className: "w-px",
              headerClassName: "w-px",
              render: (day: TraegerClosingDay) => (
                <div className="flex justify-end">
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${day.reason}`}
                    items={[
                      {
                        label: "Ändern",
                        icon: <Pencil className="h-4 w-4" aria-hidden />,
                        onClick: () => beginEdit(day),
                      },
                      { kind: "separator" },
                      {
                        label: "Löschen",
                        icon: <Trash2 className="h-4 w-4" aria-hidden />,
                        destructive: true,
                        onClick: () => setDeleting(day),
                      },
                    ]}
                  />
                </div>
              ),
            },
          ]
        : []),
    ],
    [beginEdit, canManage, schools],
  );

  const yearItems = [
    { value: String(thisYear), label: String(thisYear) },
    { value: String(thisYear + 1), label: String(thisYear + 1) },
  ];

  return (
    <TenantPage
      title="Schließtage und Feiertage"
      info={INFO}
      stats={
        data ? (
          <TenantPageStats
            items={[
              {
                value: closingDays.length,
                label: closingDays.length === 1 ? "Schließtag" : "Schließtage",
              },
              { value: holidayCount, label: "Feiertage" },
              {
                value: schools.length,
                label: schools.length === 1 ? "Schule" : "Schulen",
              },
            ]}
          />
        ) : null
      }
      statsLoading={isLoading && !data}
      error={loadError}
      searchSlot={
        <SegmentedControl
          ariaLabel="Jahr"
          items={yearItems}
          value={String(year)}
          onChange={(next) => setYear(Number(next))}
        />
      }
      actions={
        canManage ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={beginCreate}
            disabled={schools.length === 0}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Schließtag anlegen
          </Button>
        ) : null
      }
      overlays={
        <>
          <TraegerClosingDayModal
            isOpen={modalOpen}
            onClose={() => setModalOpen(false)}
            onSaved={(day) => {
              toastSuccess(`Der Schließtag „${day.reason}“ ist gespeichert.`);
              void mutate();
            }}
            initial={editing}
            schools={schools}
            defaultDate={year === thisYear ? today : from}
          />
          <ConfirmDeleteModal
            isOpen={deleting !== null}
            title="Schließtag löschen"
            description={
              deleting ? (
                <p>
                  Der Schließtag „{deleting.reason}“ (
                  {formatClosingRange(deleting)}) wird gelöscht. An diesem Tag
                  gibt es dann wieder Betreuung, Termine und Soll wie sonst.
                </p>
              ) : null
            }
            gate={{ mode: "twoStep" }}
            onConfirm={handleDelete}
            onClose={() => {
              deleteErrors.clear();
              setDeleting(null);
            }}
            loading={deleteLoading}
            error={deleteErrors.error}
          />
        </>
      }
    >
      <div className="flex flex-col gap-6">
        <SectionCard
          title="Schließtage des Trägers"
          concept={closingDaysConcept}
          description="An diesen Tagen ist die OGS der genannten Schulen zu."
          loading={isLoading && !data}
        >
          {closingDays.length === 0 ? (
            <EmptyState
              icon={
                <ConceptIconTile concept={closingDaysConcept} size="page" />
              }
              title={`Keine Schließtage in ${year}`}
              description={
                canManage
                  ? "Legen Sie oben einen Schließtag an, zum Beispiel einen Brückentag."
                  : "Das Träger-Büro hat für dieses Jahr keine Schließtage geplant."
              }
            />
          ) : (
            <DataTable
              columns={columns}
              rows={[...closingDays]}
              getRowKey={(day) => day.id}
              defaultSortKey="range"
              defaultSortDirection="asc"
              stackedOnMobile
              stackedLayout="row"
            />
          )}
        </SectionCard>

        <SectionCard
          title={`Jahresübersicht ${year}`}
          concept={calendarConcept}
          description="Feiertage und Schließtage aller Schulen auf einen Blick."
          loading={isLoading && !data}
        >
          <TraegerYearCalendar year={year} marks={marks} todayISO={today} />
        </SectionCard>

        <SectionCard
          title="Feiertage nach Bundesland"
          concept={calendarConcept}
          description="Nur zur Information. Stimmt das Bundesland einer Schule nicht, schreiben Sie dem moto-Team."
          loading={isLoading && !data}
        >
          <div
            className={
              regions.length > 1
                ? "grid grid-cols-1 gap-6 lg:grid-cols-2"
                : "grid grid-cols-1 gap-6"
            }
          >
            {regions.map((region) => (
              <section key={region.code} aria-label={region.name}>
                <div className="mb-3 flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-semibold text-gray-900">
                    {region.name}
                  </h3>
                  <StatusBadge
                    label={`${region.holidays.length} Feiertage`}
                    tone="blue"
                  />
                </div>
                <p className="mb-3 text-xs text-gray-500">
                  Gilt für:{" "}
                  {schoolsInState(schools, region.code)
                    .map((school) => school.name)
                    .join(", ")}
                </p>
                <ul className="divide-y divide-gray-100">
                  {region.holidays.map((holiday) => (
                    <li
                      key={holiday.date}
                      className="flex items-baseline justify-between gap-3 py-2 text-sm"
                    >
                      <span className="text-gray-900">{holiday.name}</span>
                      <span className="whitespace-nowrap text-gray-500 tabular-nums">
                        {formatDayWithWeekday(holiday.date)}
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        </SectionCard>
      </div>
    </TenantPage>
  );
}
