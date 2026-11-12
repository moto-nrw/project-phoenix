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
import {
  ClosingDayHolidayImpact,
  familiesLabel,
} from "~/components/traeger/closing-day-holiday-impact";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { TraegerHolidayClosureNotices } from "~/components/traeger/traeger-holiday-closure-notices";
import { useClosingDayHolidayImpact } from "~/components/traeger/use-closing-day-holiday-impact";
import { ClosingDayModal } from "~/components/planning/closing-day-modal";
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
import { ApiError } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import { calendarConcept } from "~/lib/concepts/calendar";
import { closingDaysConcept } from "~/lib/concepts/closingDays";
import { sendingRefreshInterval } from "~/lib/holiday-closure-notices";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  createTraegerClosingDay,
  updateTraegerClosingDay,
  deleteTraegerClosingDay,
  fetchPendingHolidayClosures,
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

  // Elternhinweise zu Schließtagen, die Schulen in gebuchte Ferienwochen
  // gelegt haben (#6434, ADR 0196). Solange einer gesendet wird, lädt die
  // Liste regelmäßig neu.
  const pendingResponse = useSWRAuth(
    session?.user.scope === "org" && canManage
      ? traegerKeys.holidayClosures()
      : null,
    fetchPendingHolidayClosures,
    {
      refreshInterval: (latest) => sendingRefreshInterval(latest?.notices),
    },
  );
  const pending = pendingResponse.data;
  const pendingLoadError = useSwrLoadError(
    pendingResponse.error,
    "die offenen Hinweise an Eltern",
    pendingResponse.mutate,
  );
  const holiday = useClosingDayHolidayImpact(modalOpen, editing);
  // Beim Löschen zeigt der Dialog die Tage, die wieder öffnen (#6457).
  const deleteHoliday = useClosingDayHolidayImpact(
    deleting !== null,
    deleting,
    "delete",
  );

  const schools = useMemo(() => data?.schools ?? [], [data]);
  const schoolName = useCallback(
    (id: string) => schools.find((school) => school.id === id)?.name ?? "–",
    [schools],
  );
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
      const families = await deleteTraegerClosingDay(
        deleting.id,
        deleteHoliday.impact?.token ?? "",
      );
      toastSuccess(
        families > 0
          ? `Der Schließtag „${deleting.reason}“ ist gelöscht. ${familiesLabel(families)} ${families === 1 ? "bekommt" : "bekommen"} eine E-Mail.`
          : `Der Schließtag „${deleting.reason}“ ist gelöscht.`,
      );
      setDeleting(null);
      void mutate();
      void pendingResponse.mutate();
    } catch (err) {
      logger.error("traeger_closing_day_delete_failed", {
        closing_day_id: deleting.id,
        error: err instanceof Error ? err.message : String(err),
      });
      // Haben sich die Buchungen seit der Vorschau geändert, zeigt der
      // Dialog den neuen Stand; die Meldung sagt es.
      if (
        err instanceof ApiError &&
        err.code === "traeger.closing_day_bookings_changed"
      ) {
        deleteHoliday.reload();
      }
      await deleteErrors.show(err, {
        object: "das Löschen des Schließtags",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeleteLoading(false);
    }
  }, [
    deleting,
    deleteErrors,
    deleteHoliday,
    mutate,
    pendingResponse,
    toastSuccess,
  ]);
  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDelete();
  });

  const impactFamilies = holiday.impact?.families ?? 0;

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
        stacked: "field",
        render: (day) => (
          <span className="wrap-anywhere text-gray-900">{day.reason}</span>
        ),
      },
      {
        key: "schools",
        header: "Gilt für",
        stacked: "field",
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
                        description: "Öffnet den Schließtag zum Bearbeiten.",
                        onClick: () => beginEdit(day),
                      },
                      { kind: "separator" },
                      {
                        label: "Löschen",
                        icon: <Trash2 className="h-4 w-4" aria-hidden />,
                        description:
                          "moto fragt vorher nach und nennt, wer eine E-Mail bekommt.",
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
            tooltip="Plant einen Schließtag für eine, mehrere oder alle Schulen."
            disabledReason={
              data
                ? "Ihrem Träger ist noch keine Schule zugeordnet. Das übernimmt das moto-Team."
                : undefined
            }
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Schließtag anlegen
          </Button>
        ) : null
      }
      overlays={
        <>
          <ClosingDayModal
            isOpen={modalOpen}
            onClose={() => setModalOpen(false)}
            onSave={async (input) => {
              const withToken = {
                ...input,
                holidayBookingsToken: holiday.impact?.token ?? "",
              };
              try {
                const day = editing
                  ? await updateTraegerClosingDay(editing.id, withToken)
                  : await createTraegerClosingDay(withToken);
                toastSuccess(
                  day.notifiedFamilies > 0
                    ? `Der Schließtag „${day.reason}“ ist gespeichert. ${familiesLabel(day.notifiedFamilies)} ${day.notifiedFamilies === 1 ? "bekommt" : "bekommen"} eine E-Mail.`
                    : `Der Schließtag „${day.reason}“ ist gespeichert.`,
                );
              } catch (err) {
                // Haben sich die Buchungen seit der Vorschau geändert, zeigt
                // der Dialog den neuen Stand; die Meldung sagt es.
                if (
                  err instanceof ApiError &&
                  err.code === "traeger.closing_day_bookings_changed"
                ) {
                  holiday.reload();
                }
                throw err;
              }
            }}
            onSaved={() => {
              void mutate();
              void pendingResponse.mutate();
            }}
            onDraftChange={holiday.onDraftChange}
            consequences={
              <ClosingDayHolidayImpact
                impact={holiday.impact}
                loading={holiday.loading}
                error={holiday.error}
                schoolName={schoolName}
              />
            }
            saveLabel={
              impactFamilies > 0
                ? "Speichern und Eltern informieren"
                : "Speichern"
            }
            saveBlocked={holiday.blocked}
            reasonMaxLength={120}
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
            warningSlot={
              <ClosingDayHolidayImpact
                impact={deleteHoliday.impact}
                loading={deleteHoliday.loading}
                error={deleteHoliday.error}
                schoolName={schoolName}
                action="Löschen"
              />
            }
            confirmDisabled={deleteHoliday.blocked}
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
        {canManage ? (
          <TraegerHolidayClosureNotices
            pending={pending}
            loadError={pending ? null : pendingLoadError}
            schoolName={schoolName}
            reload={pendingResponse.mutate}
          />
        ) : null}
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
