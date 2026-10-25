"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocale, useTranslations } from "next-intl";
import { Check, UserPen, Utensils } from "lucide-react";

import { Button } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { ExportMenu } from "~/components/ui/export-menu";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { MealRegistrationChangeModal } from "~/components/meal-plan/meal-registration-change-modal";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import {
  downloadDailyMealParticipants,
  downloadWeeklyMealParticipants,
  getDailyMealParticipants,
  getWeeklyMealParticipants,
  mondayOf,
  workWeekDates,
  type DailyMealParticipant,
  type WeeklyMealList,
} from "~/lib/meal-plan-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MealParticipantList" });

const isWeekendDay = (date: Date) => date.getDay() === 0 || date.getDay() === 6;

function nextWeekday(date: string): string {
  const candidate = parseISODate(date);
  while (isWeekendDay(candidate)) {
    candidate.setDate(candidate.getDate() + 1);
  }
  return toISODate(candidate);
}

type ListView = "day" | "week";

/** One child of the Wochenliste with the days it is registered for. */
interface WeeklyMealRow extends DailyMealParticipant {
  dates: ReadonlySet<string>;
}

function compareParticipants(
  a: DailyMealParticipant,
  b: DailyMealParticipant,
): number {
  return (
    a.schoolClass.localeCompare(b.schoolClass) ||
    a.lastName.localeCompare(b.lastName) ||
    a.firstName.localeCompare(b.firstName) ||
    a.studentId.localeCompare(b.studentId)
  );
}

function weeklyRows(week: WeeklyMealList | null): WeeklyMealRow[] {
  if (!week) return [];
  const rows = new Map<string, WeeklyMealRow & { dates: Set<string> }>();
  for (const day of week.days) {
    for (const participant of day.participants) {
      const row = rows.get(participant.studentId) ?? {
        ...participant,
        dates: new Set<string>(),
      };
      row.dates.add(day.date);
      rows.set(participant.studentId, row);
    }
  }
  return [...rows.values()].sort(compareParticipants);
}

/**
 * Die Tagesliste für die Küche. Mit `canRegister` (meals:register) lädt sie
 * auch die Kinder, die nicht essen: „Anmeldung ändern“ im Kopf öffnet das
 * Änderungsfenster ohne Kind, „Anmeldung ändern…“ im Menü einer Zeile mit
 * diesem Kind.
 */
export function MealParticipantList({
  canRegister,
  canExport = true,
  onRegistrationChanged,
}: Readonly<{
  canRegister: boolean;
  canExport?: boolean;
  onRegistrationChanged?: () => void;
}>) {
  const t = useTranslations("mealParticipantList");
  const locale = useLocale();
  const datePicker = useLocalizedDatePicker();
  const today = useBerlinToday();
  const { show: showActionError } = useApiErrorDisplay();
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // „Wiederholen“ lädt bzw. exportiert mit dem aktuell gewählten Tag.
  const reloadRef = useRef<() => void>(() => undefined);
  const retryDownloadRef = useRef<(format: "pdf" | "xlsx") => void>(
    () => undefined,
  );
  const [selectedView, setView] = useState<ListView>("day");
  const view = canExport ? selectedView : "day";
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const date = selectedDate ?? nextWeekday(today);
  const [deadline, setDeadline] = useState("");
  const [closed, setClosed] = useState(false);
  const [participants, setParticipants] = useState<DailyMealParticipant[]>([]);
  // Die übrigen Kinder des Tages, für Änderungen durch die Schule (#3873).
  const [others, setOthers] = useState<DailyMealParticipant[]>([]);
  // Offenes Änderungsfenster; `studentId` ist das vorgewählte Kind.
  const [changing, setChanging] = useState<{ studentId?: string } | null>(null);
  const weekStart = mondayOf(parseISODate(date));
  const [week, setWeek] = useState<WeeklyMealList | null>(null);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"pdf" | "xlsx" | null>(null);
  const errorObject = view === "week" ? t("weekErrorObject") : t("errorObject");

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      if (view === "week") {
        setWeek(await getWeeklyMealParticipants(weekStart));
      } else {
        const list = await getDailyMealParticipants(date, canRegister);
        setParticipants(list.participants);
        setOthers(list.others);
        setDeadline(list.deadline);
        setClosed(list.closed);
      }
    } catch (cause) {
      logger.error("meal_participant_list_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      setParticipants([]);
      setOthers([]);
      setWeek(null);
      void showLoadError(cause, {
        object: errorObject,
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [
    canRegister,
    view,
    date,
    weekStart,
    errorObject,
    showLoadError,
    clearLoadError,
  ]);
  useLayoutEffect(() => {
    reloadRef.current = () => void load();
  });

  useEffect(() => {
    void load();
  }, [load]);

  // Ändern geht nur in der Tagesansicht, für einen geöffneten Tag ab heute,
  // und erst, wenn die Liste geladen ist.
  const canChangeDay =
    canRegister &&
    view === "day" &&
    !loading &&
    !loadError &&
    !closed &&
    date >= today;

  const columns = useMemo<DataTableColumn<DailyMealParticipant>[]>(
    () => [
      {
        key: "name",
        header: t("child"),
        render: (row) => `${row.lastName}, ${row.firstName}`,
        sortValue: (row) => `${row.lastName} ${row.firstName}`,
        stacked: "title",
      },
      {
        key: "class",
        header: t("class"),
        render: (row) => row.schoolClass || "–",
        sortValue: (row) => row.schoolClass,
        stacked: "meta",
      },
    ],
    [t],
  );
  const dayColumns = useMemo<DataTableColumn<DailyMealParticipant>[]>(
    () =>
      canChangeDay
        ? [
            ...columns,
            {
              key: "actions",
              header: "",
              align: "right",
              stacked: "meta",
              render: (row) => (
                <OverflowMenu
                  ariaLabel={t("rowActions", {
                    name: `${row.firstName} ${row.lastName}`,
                  })}
                  items={[
                    {
                      label: t("change"),
                      onClick: () => setChanging({ studentId: row.studentId }),
                    },
                  ]}
                />
              ),
            },
          ]
        : columns,
    [canChangeDay, columns, t],
  );

  const weekDates = useMemo(
    () => week?.days.map((day) => day.date) ?? [],
    [week],
  );
  const weekRows = useMemo(() => weeklyRows(week), [week]);
  const closedDates = useMemo(
    () =>
      new Set(week?.days.filter((day) => day.closed).map((day) => day.date)),
    [week],
  );
  const weekColumns = useMemo<DataTableColumn<WeeklyMealRow>[]>(() => {
    const dayLabel = new Intl.DateTimeFormat(locale, {
      weekday: "short",
      day: "2-digit",
      month: "2-digit",
    });
    return [
      ...columns,
      ...weekDates.map((day): DataTableColumn<WeeklyMealRow> => ({
        key: day,
        header: dayLabel.format(parseISODate(day)),
        align: "center",
        stacked: "field",
        render: (row) =>
          closedDates.has(day) ? (
            <span className="text-xs text-gray-500">{t("weekClosed")}</span>
          ) : row.dates.has(day) ? (
            <>
              <Check
                className="text-moto-green-strong inline-block h-4 w-4"
                aria-hidden="true"
              />
              <span className="sr-only">{t("weekRegistered")}</span>
            </>
          ) : (
            <>
              <span aria-hidden="true" className="text-gray-400">
                –
              </span>
              <span className="sr-only">{t("weekNotRegistered")}</span>
            </>
          ),
        sortValue: (row) => (row.dates.has(day) ? 1 : 0),
      })),
    ];
  }, [columns, weekDates, closedDates, locale, t]);

  const isWeek = view === "week";
  const weekdayShort = new Intl.DateTimeFormat(locale, { weekday: "short" });
  const weekEnd = workWeekDates(weekStart).at(-1) ?? weekStart;

  async function download(format: "pdf" | "xlsx") {
    setExporting(format);
    try {
      if (view === "week") {
        await downloadWeeklyMealParticipants(weekStart, format);
      } else {
        await downloadDailyMealParticipants(date, format);
      }
    } catch (downloadError) {
      logger.error("meal_participant_list_export_failed", {
        error:
          downloadError instanceof Error
            ? downloadError.message
            : String(downloadError),
      });
      void showActionError(downloadError, {
        object: errorObject,
        retry: () => retryDownloadRef.current(format),
      });
    } finally {
      setExporting(null);
    }
  }
  useLayoutEffect(() => {
    retryDownloadRef.current = (format) => void download(format);
  });

  const cutoffLine =
    isWeek || (!closed && !deadline)
      ? null
      : closed
        ? t("closedDay")
        : t("cutoff", {
            date: new Date(deadline).toLocaleString(locale, {
              weekday: "short",
              day: "2-digit",
              month: "2-digit",
              hour: "2-digit",
              minute: "2-digit",
              timeZone: "Europe/Berlin",
            }),
          });

  // Eine Karte wie „Bestellung“: Titel, Frist und Aktionen im Kopf, die
  // Liste der Kinder als Körper (#3873). Die Kopfzeile bricht selbst um.
  return (
    <SectionCard
      title={isWeek ? t("weekTitle") : t("title")}
      description={
        <>
          <span className="block">
            {isWeek ? t("weekDescription") : t("description")}
          </span>
          {cutoffLine ? (
            <span className="mt-1 block font-medium text-gray-700">
              {cutoffLine}
            </span>
          ) : null}
        </>
      }
      actions={
        <>
          {canExport ? (
            <SegmentedControl
              ariaLabel={t("viewAria")}
              items={[
                { value: "day", label: t("viewDay") },
                { value: "week", label: t("viewWeek") },
              ]}
              value={view}
              onChange={setView}
            />
          ) : null}
          <ISODatePicker
            id="meal-participant-date"
            value={isWeek ? weekStart : date}
            onChange={setSelectedDate}
            ariaLabel={
              isWeek
                ? t("weekDateAria", {
                    from: formatDate(weekStart, false, locale),
                  })
                : t("dateAria", {
                    date: formatDate(date, false, locale),
                  })
            }
            {...datePicker}
            calendarLayout="popover-below"
            controlSize="md"
            disabledDay={isWeekendDay}
            hideClearButton
            required
            className="w-full sm:w-44"
            triggerClassName="w-full min-w-44 flex-none justify-between rounded-lg text-sm font-medium"
          />

          {canRegister && !isWeek ? (
            <Button
              type="button"
              variant="primary"
              size="md"
              className="gap-2"
              onClick={() => setChanging({})}
              disabled={!canChangeDay}
            >
              <UserPen className="h-4 w-4" aria-hidden="true" />
              {t("changeTitle")}
            </Button>
          ) : null}

          {canExport ? (
            <>
              <ExportMenu
                label={isWeek ? t("weekDownloadGroup") : t("downloadGroup")}
                options={[
                  { key: "pdf", format: "pdf", label: t("pdf") },
                  { key: "xlsx", format: "xlsx", label: t("excel") },
                ]}
                busyKey={exporting}
                unavailable={loading || Boolean(loadError)}
                onSelect={(option) => {
                  if (option.format === "pdf" || option.format === "xlsx")
                    void download(option.format);
                }}
              />
              <span className="sr-only" role="status" aria-live="polite">
                {exporting === "pdf"
                  ? t("pdfDownloadStatus")
                  : exporting === "xlsx"
                    ? t("excelDownloadStatus")
                    : ""}
              </span>
            </>
          ) : null}
        </>
      }
    >
      {isWeek && week && !loading && !loadError ? (
        <p className="mb-4 text-sm text-gray-700">
          <span className="text-gray-500">{t("weekCountsLabel")}: </span>
          <span className="font-semibold tabular-nums">
            {week.days
              .map(
                (day) =>
                  `${weekdayShort.format(parseISODate(day.date))} ${
                    day.closed
                      ? t("weekClosed")
                      : t("weekCount", { count: day.participants.length })
                  }`,
              )
              .join(" · ")}
          </span>
        </p>
      ) : null}

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : isWeek ? (
        <DataTable
          columns={weekColumns}
          rows={weekRows}
          getRowKey={(row) => row.studentId}
          isLoading={loading}
          stackedOnMobile
          caption={t("weekCaption", {
            from: formatDate(weekStart, false, locale),
            to: formatDate(weekEnd, false, locale),
          })}
          emptyState={
            <EmptyState
              icon={<Utensils className="h-6 w-6" />}
              title={t("weekEmptyTitle")}
              description={t("weekEmptyDescription")}
            />
          }
        />
      ) : (
        <DataTable
          columns={dayColumns}
          rows={participants}
          getRowKey={(row) => row.studentId}
          isLoading={loading}
          stackedOnMobile
          rowHasInteractiveControls={canChangeDay}
          caption={t("caption", {
            date: formatDate(date, false, locale),
          })}
          emptyState={
            <EmptyState
              icon={<Utensils className="h-6 w-6" />}
              title={t("emptyTitle")}
              description={t("emptyDescription")}
            />
          }
        />
      )}
      <MealRegistrationChangeModal
        isOpen={canRegister && changing !== null}
        onClose={() => setChanging(null)}
        onSaved={() => {
          void load();
          onRegistrationChanged?.();
        }}
        date={date}
        participants={participants}
        others={others}
        initialStudentId={changing?.studentId}
      />
    </SectionCard>
  );
}
