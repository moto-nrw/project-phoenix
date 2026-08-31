"use client";

import React, {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChevronRight } from "lucide-react";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { useTenantRouter } from "~/lib/tenant-router";
import { BackButton } from "~/components/ui/back-button";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  ConceptPageHeader,
  ConceptSectionHeader,
} from "~/components/ui/concept-section-header";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { useScrollToTop } from "~/lib/hooks/use-scroll-to-top";
import { createLogger } from "~/lib/logger";
import { todayISO } from "~/lib/date-helpers";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import {
  type AttendanceHistory,
  type AttendanceHistoryDay,
  type BackendAttendanceHistoryResponse,
  formatDate,
  formatAttendanceSlotStatus,
  formatDuration,
  formatTime,
  mapAttendanceHistoryResponse,
} from "~/lib/attendance-history-helpers";
import {
  AttendanceCorrectionModal,
  type CorrectableSlot,
} from "~/components/students/attendance-correction-modal";
import { hasPermission } from "~/lib/auth-utils";
import { RoomHistorySkeleton } from "./page-skeleton";

const logger = createLogger({ component: "StudentRoomHistoryPage" });

const EXPORT_FORMATS = ["pdf", "docx", "xlsx"] as const;
type ExportFormat = (typeof EXPORT_FORMATS)[number];

// ─── Types ───────────────────────────────────────────────────────────────────

interface Student {
  id: string;
  first_name: string;
  second_name: string;
  name?: string;
  school_class: string;
  group_id?: string;
  group_name?: string;
}

type ErrorCode =
  "feature_disabled" | "not_group_supervisor" | "not_found" | "generic";

const ERROR_MESSAGES: Record<ErrorCode, string> = {
  feature_disabled:
    "Diese Funktion ist für Ihre Schule deaktiviert. Bitte wenden Sie sich an Ihre Administration.",
  not_group_supervisor:
    "Ihr Konto ist keinem Personaleintrag zugeordnet. Bitte wenden Sie sich an Ihre Administration.",
  not_found: "Kind nicht gefunden.",
  generic: "Fehler beim Laden des Anwesenheitsprotokolls.",
};

// ─── Chart config ────────────────────────────────────────────────────────────

const durationChartConfig: ChartConfig = {
  duration: {
    label: "Stunden",
    color: MOTO_COLOR_PALETTE.green.base,
  },
};

const activityChartConfig: ChartConfig = {
  visits: {
    label: "Raumwechsel",
    color: MOTO_COLOR_PALETTE.blue.base,
  },
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatDateShort(dateKey: string): string {
  const d = new Date(`${dateKey}T00:00:00`);
  const day = d.getDate().toString().padStart(2, "0");
  const month = (d.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}`;
}

function formatWeekday(dateKey: string): string {
  const d = new Date(`${dateKey}T00:00:00`);
  return d.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    weekday: "short",
  });
}

// ─── Shared chart tick ───────────────────────────────────────────────────────

function TodayTick({
  chartData,
  props,
}: {
  readonly chartData: ReadonlyArray<{ date: string; isToday: boolean }>;
  readonly props: Record<string, unknown>;
}) {
  const x = Number(props.x);
  const y = Number(props.y);
  const idx = (props.payload as { index?: number })?.index ?? 0;
  const item = chartData[idx];
  const isToday = item?.isToday;
  return (
    <g>
      <text
        x={x}
        y={y + 12}
        textAnchor="middle"
        fontSize={11}
        fontWeight={isToday ? 700 : 400}
        fill={
          isToday
            ? MOTO_COLOR_PALETTE.neutral.strong
            : MOTO_COLOR_PALETTE.neutral.light
        }
      >
        {item?.date}
      </text>
      {isToday && (
        <text
          x={x}
          y={y + 24}
          textAnchor="middle"
          fontSize={9}
          fontWeight={500}
          fill={MOTO_COLOR_PALETTE.green.base}
        >
          heute
        </text>
      )}
    </g>
  );
}

// ─── Charts ─────────────────────────────────────────────────────────────────

function HistoryCharts({ days }: { readonly days: AttendanceHistoryDay[] }) {
  const todayKey = todayISO();

  const chartData = useMemo(() => {
    return days
      .filter((day) => day.attendance)
      .slice()
      .reverse()
      .map((day) => ({
        date: formatDateShort(day.date),
        isToday: day.date === todayKey,
        roomDetailAvailable: day.roomDetailAvailable,
        duration: day.attendance
          ? Math.round(((day.attendance.durationMinutes ?? 0) / 60) * 10) / 10
          : 0,
        visits: day.visits.length,
      }));
  }, [days, todayKey]);

  // Filter out days where room details are unavailable (retention cap),
  // so the activity chart doesn't show misleading zero-height bars.
  const activityChartData = useMemo(
    () => chartData.filter((d) => d.roomDetailAvailable),
    [chartData],
  );
  const renderDurationTick = useCallback(
    (p: Record<string, unknown>) => (
      <TodayTick chartData={chartData} props={p} />
    ),
    [chartData],
  );
  const renderActivityTick = useCallback(
    (p: Record<string, unknown>) => (
      <TodayTick chartData={activityChartData} props={p} />
    ),
    [activityChartData],
  );
  const renderDurationTooltipValue = useCallback(
    (value: string | number) => (
      <span className="font-medium">{value} Std</span>
    ),
    [],
  );
  const renderActivityTooltipValue = useCallback(
    (value: string | number) => (
      <span className="font-medium">{value} Wechsel</span>
    ),
    [],
  );

  if (chartData.length === 0) return null;

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 md:gap-6">
      {/* Anwesenheit */}
      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="p-4 sm:p-6">
          <ConceptSectionHeader
            className="mb-3"
            title="Anwesenheit"
            concept="present"
            subtitle="Tägliche Aufenthaltsdauer in Stunden"
          />
          <ChartContainer
            config={durationChartConfig}
            className="h-[180px] w-full sm:h-[200px]"
          >
            <BarChart
              data={chartData}
              margin={{ top: 4, right: 4, bottom: 0, left: -20 }}
              barCategoryGap="20%"
            >
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                fontSize={11}
                interval={0}
                tick={renderDurationTick}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={4}
                fontSize={12}
                tickFormatter={(v: number) => `${v}h`}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    labelFormatter={(label) => `Tag: ${label}`}
                    formatter={renderDurationTooltipValue}
                  />
                }
              />
              <Bar
                dataKey="duration"
                fill="var(--color-duration)"
                radius={[6, 6, 6, 6]}
              />
            </BarChart>
          </ChartContainer>
        </div>
      </div>

      {/* Aktivität (Raumwechsel) */}
      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="p-4 sm:p-6">
          <ConceptSectionHeader
            className="mb-3"
            title="Aktivität"
            concept="rooms"
            subtitle="Raumwechsel pro Tag"
          />
          {activityChartData.length === 0 ? (
            <div className="flex h-[180px] items-center justify-center sm:h-[200px]">
              <p className="text-sm text-gray-400">
                Keine Raumdetails verfügbar (Aufbewahrungsfrist überschritten).
              </p>
            </div>
          ) : (
            <ChartContainer
              config={activityChartConfig}
              className="h-[180px] w-full sm:h-[200px]"
            >
              <BarChart
                data={activityChartData}
                margin={{ top: 4, right: 4, bottom: 0, left: -20 }}
                barCategoryGap="20%"
              >
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey="date"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  fontSize={11}
                  interval={0}
                  tick={renderActivityTick}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  tickMargin={4}
                  fontSize={12}
                  allowDecimals={false}
                />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      labelFormatter={(label) => `Tag: ${label}`}
                      formatter={renderActivityTooltipValue}
                    />
                  }
                />
                <Bar
                  dataKey="visits"
                  fill="var(--color-visits)"
                  radius={[6, 6, 6, 6]}
                />
              </BarChart>
            </ChartContainer>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── DayCard (mobile) ────────────────────────────────────────────────────────

function DayCard({
  day,
  isToday,
  canCorrect,
  onCorrect,
}: {
  readonly day: AttendanceHistoryDay;
  readonly isToday: boolean;
  readonly canCorrect: boolean;
  readonly onCorrect: (slot: CorrectableSlot) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const statusLabel = day.statusEntries.map((entry) => entry.label).join(", ");

  return (
    <div className="border-b border-gray-100 last:border-b-0">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className={`flex w-full items-center justify-between px-4 py-3 text-left transition-colors hover:bg-gray-50 ${isToday ? "bg-blue-50/50" : ""}`}
      >
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100 text-xs font-medium text-gray-600">
            {formatWeekday(day.date)}
          </div>
          <div>
            <span className="text-sm font-medium text-gray-900">
              {formatDateShort(day.date)}
            </span>
            {day.attendance && (
              <span className="ml-2 text-xs text-gray-500">
                {formatTime(day.attendance.checkInTime)} –{" "}
                {day.attendance.checkOutTime
                  ? formatTime(day.attendance.checkOutTime)
                  : "anwesend"}
              </span>
            )}
            {!day.attendance && statusLabel && (
              <span className="ml-2 text-xs font-medium text-amber-700">
                {statusLabel}
              </span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {day.attendance && (
            <span className="bg-moto-green/10 text-moto-green-strong rounded-full px-2 py-0.5 text-xs font-medium">
              {formatDuration(day.attendance.durationMinutes)}
            </span>
          )}
          {!day.attendance && (
            <span className="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700">
              {statusLabel || "Keine Daten"}
            </span>
          )}
          <ChevronRight
            className={`h-4 w-4 text-gray-400 transition-transform ${expanded ? "rotate-90" : ""}`}
          />
        </div>
      </button>

      {expanded && (
        <div className="border-t border-gray-50 bg-gray-50/50 px-4 py-3">
          {day.slots.length > 0 && (
            <div className="mb-3 space-y-2">
              <p className="text-xs font-semibold text-gray-700">
                Betreuungsangebote
              </p>
              {day.slots.map((slot) => (
                <div key={slot.instanceId} className="text-xs">
                  <div className="flex items-center justify-between">
                    <div>
                      <span className="font-medium text-gray-800">
                        {slot.title}
                      </span>
                      <span className="ml-2 text-gray-500">
                        {slot.startTime}–{slot.endTime}
                      </span>
                      {slot.isUnplanned && (
                        <span className="text-moto-orange ml-2">ungeplant</span>
                      )}
                    </div>
                    <span className="font-medium text-gray-600">
                      {formatAttendanceSlotStatus(slot.status, slot.substatus)}
                    </span>
                  </div>
                  {/* Die Bemerkung aus der Betreuung (#2898): ruhige
                      Information unter ihrem Block, nicht klickbar. */}
                  {slot.note && (
                    <p className="mt-0.5 pl-1 text-gray-500 italic">
                      Bemerkung: {slot.note}
                    </p>
                  )}
                  {/* Korrigieren nur mit Recht und nur für echte Termine:
                      synthetische "Ohne Zuordnung"-Zeilen tragen eine negative
                      Kennung und haben keinen Eintrag zum Korrigieren. */}
                  {canCorrect && !slot.instanceId.startsWith("-") && (
                    <button
                      type="button"
                      onClick={() =>
                        onCorrect({
                          instanceId: slot.instanceId,
                          title: slot.title,
                          date: formatDateShort(day.date),
                          startTime: slot.startTime,
                          endTime: slot.endTime,
                          status: slot.status,
                          substatus: slot.substatus,
                          note: slot.note,
                        })
                      }
                      className="mt-0.5 pl-1 text-gray-500 underline decoration-gray-300 underline-offset-4 hover:decoration-gray-600"
                    >
                      Korrigieren
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
          {day.statusEntries.length > 0 && (
            <div className="mb-3 space-y-1">
              {day.statusEntries.map((entry) => (
                <div
                  key={`${day.date}-${entry.status}`}
                  className="flex items-center justify-between text-xs"
                >
                  <span className="font-medium text-amber-800">
                    {entry.label}
                  </span>
                  <span className="text-gray-500">
                    gemeldet {formatTime(entry.reportedAt)}
                    {entry.clearedAt
                      ? ` · beendet ${formatTime(entry.clearedAt)}`
                      : ""}
                  </span>
                </div>
              ))}
            </div>
          )}
          {!day.roomDetailAvailable ? (
            <p className="text-xs text-gray-500 italic">
              Raumdetails nicht mehr verfügbar (Aufbewahrungsfrist
              überschritten).
            </p>
          ) : day.visits.length === 0 ? (
            <p className="text-xs text-gray-500">
              Keine Raumwechsel an diesem Tag.
            </p>
          ) : (
            <div className="space-y-2">
              {day.visits.map((v, i) => (
                <div
                  key={`${day.date}-${i}-${v.entryTime.toISOString()}`}
                  className="flex items-center justify-between text-xs"
                >
                  <div className="flex items-center gap-2">
                    <div className="bg-moto-blue h-1.5 w-1.5 rounded-full" />
                    <span className="font-medium text-gray-700">
                      {v.roomName || "Unbekannt"}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 text-gray-500">
                    <span className="tabular-nums">
                      {formatTime(v.entryTime)}
                      {v.exitTime && <> – {formatTime(v.exitTime)}</>}
                    </span>
                    {v.durationMinutes != null && (
                      <span className="rounded bg-gray-100 px-1.5 py-0.5 tabular-nums">
                        {formatDuration(v.durationMinutes)}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ─── HistoryTable (desktop) ──────────────────────────────────────────────────

function HistoryTable({
  days,
  caps,
  studentId,
  onCorrected,
}: {
  readonly days: AttendanceHistoryDay[];
  readonly caps: { attendanceDays: number; roomDetailDays: number };
  readonly studentId: string;
  readonly onCorrected: () => void;
}) {
  const [expandedDate, setExpandedDate] = useState<string | null>(null);
  const todayKey = todayISO();
  // Korrigieren darf nur, wer den Plan verwaltet (#2898) — dieselbe
  // Berechtigung, die das Backend auf der Korrektur-Route verlangt.
  const { data: session } = useSession();
  const canCorrect = hasPermission(session, "schedules:manage");
  const [correcting, setCorrecting] = useState<CorrectableSlot | null>(null);

  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="border-b border-gray-100 px-4 py-3 sm:px-6 sm:py-4">
        <ConceptSectionHeader
          title="Anwesenheitsprotokoll"
          concept="changeHistory"
          subtitle={
            <>
              Letzte {caps.attendanceDays} Tage · Raumdetails für{" "}
              {caps.roomDetailDays} Tage
            </>
          }
        />
      </div>

      {days.length === 0 ? (
        <div className="px-6 py-12 text-center">
          <p className="text-sm text-gray-500">
            Keine Anwesenheitsdaten für den ausgewählten Zeitraum verfügbar.
          </p>
        </div>
      ) : (
        <>
          {/* Desktop table */}
          <div className="hidden md:block">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-100 text-left text-xs font-medium text-gray-500">
                  <th className="px-6 py-3">Tag</th>
                  <th className="px-6 py-3">Ankunft</th>
                  <th className="px-6 py-3">Abmeldung</th>
                  <th className="px-6 py-3">Dauer</th>
                  <th className="px-6 py-3">Angebote / Räume</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {days.map((day) => {
                  const isExpanded = expandedDate === day.date;
                  const isToday = day.date === todayKey;
                  const statusLabel = day.statusEntries
                    .map((entry) => entry.label)
                    .join(", ");
                  return (
                    <React.Fragment key={day.date}>
                      <tr
                        onClick={() =>
                          setExpandedDate(isExpanded ? null : day.date)
                        }
                        onKeyDown={(event) => {
                          if (event.key !== "Enter" && event.key !== " ")
                            return;
                          event.preventDefault();
                          setExpandedDate(isExpanded ? null : day.date);
                        }}
                        tabIndex={0}
                        aria-expanded={isExpanded}
                        aria-label={`${formatDate(day.date)}: Details ${isExpanded ? "schließen" : "öffnen"}`}
                        className={`cursor-pointer text-sm transition-colors hover:bg-gray-50 ${isToday ? "bg-blue-50/50" : ""}`}
                      >
                        <td className="px-6 py-3">
                          <div className="flex items-center gap-2">
                            <ChevronRight
                              className={`h-3.5 w-3.5 text-gray-400 transition-transform ${isExpanded ? "rotate-90" : ""}`}
                            />
                            <span className="font-medium text-gray-900">
                              {formatDate(day.date)}
                            </span>
                            {statusLabel && (
                              <span className="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700">
                                {statusLabel}
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="px-6 py-3 text-gray-600 tabular-nums">
                          {day.attendance
                            ? formatTime(day.attendance.checkInTime)
                            : "–"}
                        </td>
                        <td className="px-6 py-3 text-gray-600 tabular-nums">
                          {day.attendance
                            ? day.attendance.checkOutTime
                              ? formatTime(day.attendance.checkOutTime)
                              : "Noch anwesend"
                            : "–"}
                        </td>
                        <td className="px-6 py-3">
                          {day.attendance ? (
                            <span className="bg-moto-green/10 text-moto-green-strong rounded-full px-2.5 py-0.5 text-xs font-medium">
                              {formatDuration(day.attendance.durationMinutes)}
                            </span>
                          ) : (
                            <span className="text-gray-400">–</span>
                          )}
                        </td>
                        <td className="px-6 py-3 text-gray-500">
                          {`${day.slots.length} Angebot${day.slots.length !== 1 ? "e" : ""}`}
                          {day.roomDetailAvailable &&
                            ` · ${day.visits.length} Raum${day.visits.length !== 1 ? "wechsel" : ""}`}
                        </td>
                      </tr>

                      {/* Expanded care-offering slots */}
                      {isExpanded &&
                        day.slots.map((slot) => (
                          <tr
                            key={`${day.date}-slot-${slot.instanceId}`}
                            className="bg-gray-50/70 text-xs"
                          >
                            <td className="py-2 pr-6 pl-12">
                              <div className="flex items-center gap-2">
                                <div className="bg-moto-green h-1.5 w-1.5 shrink-0 rounded-full" />
                                <span className="font-medium text-gray-700">
                                  {slot.title}
                                </span>
                                <span className="text-gray-400 tabular-nums">
                                  {slot.startTime}
                                  {slot.endTime && <>–{slot.endTime}</>}
                                </span>
                                {slot.isUnplanned && (
                                  <span className="text-moto-orange">
                                    ungeplant
                                  </span>
                                )}
                              </div>
                              {/* Die Bemerkung aus der Betreuung (#2898):
                                  ruhige Information unter ihrem Block, nicht
                                  klickbar. Korrigieren ist die einzige Aktion
                                  und nur mit Recht sichtbar; synthetische
                                  "Ohne Zuordnung"-Zeilen tragen eine negative
                                  Kennung und haben nichts zu korrigieren. */}
                              {slot.note && (
                                <p className="mt-1 pl-4 text-gray-500 italic">
                                  Bemerkung: {slot.note}
                                </p>
                              )}
                              {canCorrect &&
                                !slot.instanceId.startsWith("-") && (
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setCorrecting({
                                        instanceId: slot.instanceId,
                                        title: slot.title,
                                        date: formatDateShort(day.date),
                                        startTime: slot.startTime,
                                        endTime: slot.endTime,
                                        status: slot.status,
                                        substatus: slot.substatus,
                                        note: slot.note,
                                      })
                                    }
                                    className="mt-1 pl-4 text-gray-500 underline decoration-gray-300 underline-offset-4 hover:decoration-gray-600"
                                  >
                                    Korrigieren
                                  </button>
                                )}
                            </td>
                            <td className="px-6 py-2 text-gray-500 tabular-nums">
                              {slot.checkedInAt
                                ? formatTime(slot.checkedInAt)
                                : "–"}
                            </td>
                            <td className="px-6 py-2 text-gray-500 tabular-nums">
                              {slot.checkedOutAt
                                ? formatTime(slot.checkedOutAt)
                                : "–"}
                            </td>
                            <td className="px-6 py-2 text-gray-600">
                              {formatAttendanceSlotStatus(
                                slot.status,
                                slot.substatus,
                              )}
                            </td>
                            <td className="px-6 py-2" />
                          </tr>
                        ))}

                      {/* Expanded room visits */}
                      {isExpanded &&
                        (!day.roomDetailAvailable ? (
                          <tr>
                            <td
                              colSpan={5}
                              className="bg-gray-50/70 px-6 py-3 text-xs text-gray-500 italic"
                            >
                              Raumdetails nicht mehr verfügbar
                              (Aufbewahrungsfrist überschritten).
                            </td>
                          </tr>
                        ) : day.visits.length === 0 ? (
                          <>
                            {day.statusEntries.map((entry) => (
                              <tr
                                key={`${day.date}-${entry.status}`}
                                className="bg-gray-50/70 text-xs"
                              >
                                <td className="py-2 pr-6 pl-12 font-medium text-amber-800">
                                  {entry.label}
                                </td>
                                <td className="px-6 py-2 text-gray-500 tabular-nums">
                                  {formatTime(entry.reportedAt)}
                                </td>
                                <td className="px-6 py-2 text-gray-500 tabular-nums">
                                  {entry.clearedAt
                                    ? formatTime(entry.clearedAt)
                                    : "–"}
                                </td>
                                <td className="px-6 py-2 text-gray-400">–</td>
                                <td className="px-6 py-2" />
                              </tr>
                            ))}
                            <tr>
                              <td
                                colSpan={5}
                                className="bg-gray-50/70 px-6 py-3 text-xs text-gray-500"
                              >
                                Keine Raumwechsel an diesem Tag.
                              </td>
                            </tr>
                          </>
                        ) : (
                          <>
                            {day.statusEntries.map((entry) => (
                              <tr
                                key={`${day.date}-${entry.status}`}
                                className="bg-gray-50/70 text-xs"
                              >
                                <td className="py-2 pr-6 pl-12 font-medium text-amber-800">
                                  {entry.label}
                                </td>
                                <td className="px-6 py-2 text-gray-500 tabular-nums">
                                  {formatTime(entry.reportedAt)}
                                </td>
                                <td className="px-6 py-2 text-gray-500 tabular-nums">
                                  {entry.clearedAt
                                    ? formatTime(entry.clearedAt)
                                    : "–"}
                                </td>
                                <td className="px-6 py-2 text-gray-400">–</td>
                                <td className="px-6 py-2" />
                              </tr>
                            ))}
                            {day.visits.map((v, i) => (
                              <tr
                                key={`${day.date}-${i}-${v.entryTime.toISOString()}`}
                                className="border-b border-gray-50 bg-gray-50/70 text-xs"
                              >
                                <td className="py-2 pr-6 pl-12">
                                  <div className="flex items-center gap-2">
                                    <div className="bg-moto-blue h-1.5 w-1.5 shrink-0 rounded-full" />
                                    <span className="font-medium text-gray-700">
                                      {v.roomName || "Unbekannt"}
                                    </span>
                                  </div>
                                </td>
                                <td className="px-6 py-2 text-gray-500 tabular-nums">
                                  {formatTime(v.entryTime)}
                                </td>
                                <td className="px-6 py-2 text-gray-500 tabular-nums">
                                  {v.exitTime ? formatTime(v.exitTime) : "–"}
                                </td>
                                <td className="px-6 py-2">
                                  {v.durationMinutes != null ? (
                                    <span className="rounded bg-gray-200/60 px-1.5 py-0.5 text-gray-600 tabular-nums">
                                      {formatDuration(v.durationMinutes)}
                                    </span>
                                  ) : (
                                    <span className="text-gray-400">–</span>
                                  )}
                                </td>
                                <td className="px-6 py-2" />
                              </tr>
                            ))}
                          </>
                        ))}
                    </React.Fragment>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Mobile card layout */}
          <div className="md:hidden">
            {days.map((day) => (
              <DayCard
                key={day.date}
                day={day}
                isToday={day.date === todayKey}
                canCorrect={canCorrect}
                onCorrect={setCorrecting}
              />
            ))}
          </div>
        </>
      )}
      {correcting && (
        <AttendanceCorrectionModal
          isOpen
          onClose={() => setCorrecting(null)}
          studentId={studentId}
          slot={correcting}
          onCorrected={onCorrected}
        />
      )}
    </div>
  );
}

// ─── Page ────────────────────────────────────────────────────────────────────

export default function StudentRoomHistoryPage() {
  return (
    <Suspense fallback={null}>
      <StudentRoomHistoryPageContent />
    </Suspense>
  );
}

function StudentRoomHistoryPageContent() {
  const router = useTenantRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  useSession();

  const [student, setStudent] = useState<Student | null>(null);
  const [history, setHistory] = useState<AttendanceHistory | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorCode, setErrorCode] = useState<ErrorCode | null>(null);
  const [exporting, setExporting] = useState<ExportFormat | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

  // Start at the top instead of inheriting the previous page's scroll position
  useScrollToTop(studentId);

  const fetchStudent = useCallback(async (): Promise<Student | null> => {
    try {
      const res = await fetch(`/api/students/${studentId}`);
      if (!res.ok) return null;
      const body = (await res.json()) as { data?: Student };
      return body.data ?? null;
    } catch (err) {
      logger.error("student_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      return null;
    }
  }, [studentId]);

  const fetchHistory = useCallback(async (): Promise<void> => {
    try {
      const res = await fetch(`/api/students/${studentId}/attendance-history`);
      if (res.status === 403) {
        const body = (await res.json()) as { error?: string };
        setErrorCode(
          body.error === "not_group_supervisor"
            ? "not_group_supervisor"
            : "feature_disabled",
        );
        setHistory(null);
        return;
      }
      if (res.status === 404) {
        setErrorCode("not_found");
        return;
      }
      if (!res.ok) {
        setErrorCode("generic");
        return;
      }
      const body = (await res.json()) as {
        data: BackendAttendanceHistoryResponse;
      };
      setHistory(mapAttendanceHistoryResponse(body.data));
      setErrorCode(null);
    } catch (err) {
      logger.error("attendance_history_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      setErrorCode("generic");
    }
  }, [studentId]);

  const downloadExport = useCallback(
    async (format: ExportFormat): Promise<void> => {
      setExporting(format);
      setExportError(null);
      try {
        const res = await fetch(
          `/api/students/${studentId}/attendance-history/export?format=${format}`,
        );
        if (!res.ok) {
          setExportError(
            "Export fehlgeschlagen. Bitte versuchen Sie es später erneut.",
          );
          return;
        }
        const blob = await res.blob();
        const disposition = res.headers.get("Content-Disposition") ?? "";
        const filename =
          /filename="([^"]+)"/.exec(disposition)?.[1] ??
          `anwesenheit.${format}`;
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = filename;
        document.body.appendChild(anchor);
        anchor.click();
        anchor.remove();
        URL.revokeObjectURL(url);
      } catch (err) {
        logger.error("attendance_history_export_failed", {
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        setExportError(
          "Export fehlgeschlagen. Bitte versuchen Sie es später erneut.",
        );
      } finally {
        setExporting(null);
      }
    },
    [studentId],
  );

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    void (async () => {
      const [s] = await Promise.all([fetchStudent(), fetchHistory()]);
      if (cancelled) return;
      setStudent(s);
      setLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  }, [fetchStudent, fetchHistory]);

  if (loading) {
    return <RoomHistorySkeleton />;
  }

  if (errorCode !== null && errorCode !== "feature_disabled") {
    return (
      <>
        <BackButton referrer={referrer} />
        <div className="flex min-h-[50vh] flex-col items-center justify-center">
          <Alert type="error" message={ERROR_MESSAGES[errorCode]} />
          <Button
            type="button"
            onClick={() => router.push(referrer)}
            variant="secondary"
            size="md"
            className="mt-4"
          >
            Zurück
          </Button>
        </div>
      </>
    );
  }

  const displayName = student
    ? (student.name ?? `${student.first_name} ${student.second_name}`)
    : "";

  return (
    <div className="w-full">
      {/* Back button (mobile only). tab=historie returns to the originating tab
          on the detail page (this sub-page lives under Historie, issue #1501);
          from= still drives the detail page's own back button to the list. */}
      <BackButton
        referrer={`/students/${studentId}?from=${referrer}&tab=historie`}
      />

      {student && (
        <ConceptPageHeader
          className="mb-6 ml-6"
          title={displayName}
          eyebrow="Anwesenheitsprotokoll"
          concept="changeHistory"
          subtitle={
            <>
              {student.school_class}
              {student.group_name ? ` · ${student.group_name}` : null}
            </>
          }
        />
      )}

      {/* Feature disabled banner */}
      {errorCode === "feature_disabled" && (
        <div className="mb-4 rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 md:mb-6">
          {ERROR_MESSAGES.feature_disabled}
        </div>
      )}

      {history && (
        <>
          {exportError && (
            <div className="mb-4">
              <Alert type="error" message={exportError} />
            </div>
          )}
          <div className="mb-4 flex flex-wrap justify-end gap-2">
            {EXPORT_FORMATS.map((format) => (
              <Button
                key={format}
                type="button"
                variant="outline"
                size="compact"
                disabled={exporting !== null}
                onClick={() => void downloadExport(format)}
              >
                {exporting === format
                  ? "Wird exportiert…"
                  : `${format.toUpperCase()} exportieren`}
              </Button>
            ))}
          </div>
          {/* Charts */}
          <div className="mb-4 md:mb-6">
            <HistoryCharts days={history.days} />
          </div>

          {/* History table */}
          <HistoryTable
            days={history.days}
            caps={history.caps}
            studentId={studentId}
            onCorrected={() => void fetchHistory()}
          />
        </>
      )}
    </div>
  );
}
