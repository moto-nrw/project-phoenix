"use client";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";

import React, {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import dynamic from "next/dynamic";
import { useParams, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { ChevronRight, Download } from "lucide-react";
import { Skeleton } from "~/components/ui/skeleton";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { SectionCard } from "~/components/ui/section-card";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  resolveStudentReferrer,
  studentHistoryBack,
} from "../student-referrer";
import { useApiLoadError } from "~/contexts/ToastContext";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { useScrollToTop } from "~/lib/hooks/use-scroll-to-top";
import { createLogger } from "~/lib/logger";

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
  attendanceActorLabel,
  type AttendanceActor,
} from "~/lib/attendance-actor";
import {
  AttendanceCorrectionModal,
  type CorrectableSlot,
} from "~/components/students/attendance-correction-modal";
import { hasPermission } from "~/lib/auth-utils";
import { RoomHistorySkeleton } from "./page-skeleton";
import type { HistoryChartPoint } from "./history-charts";
import { AttendanceExportDialog } from "~/components/students/attendance-export-dialog";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";

const logger = createLogger({ component: "StudentRoomHistoryPage" });

// ─── Types ───────────────────────────────────────────────────────────────────

interface Student {
  id: string;
  first_name: string;
  second_name: string;
  name?: string;
  school_class: string;
  group_id?: string;
  group_name?: string;
  /** Per-school switch `gdpr.attendance_log_enabled`; off answers 403. */
  attendance_log_enabled?: boolean;
}

const ROOM_HISTORY_DESCRIPTION =
  "Wo dieses Kind an einem Tag war und wer es ein- und ausgecheckt hat.";

const FEATURE_DISABLED_DESCRIPTION =
  "Das Anwesenheitsprotokoll ist für Ihre Schule ausgeschaltet. Bitte fragen Sie Ihre Administration.";

const LazyHistoryCharts = dynamic(() => import("./history-charts"), {
  ssr: false,
  loading: () => (
    <Skeleton
      className="h-[404px] w-full rounded-2xl md:h-[200px]"
      aria-hidden
    />
  ),
});

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

function HistoryCharts({ days }: { readonly days: AttendanceHistoryDay[] }) {
  const todayKey = useBerlinToday();

  const chartData = useMemo<HistoryChartPoint[]>(() => {
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

  if (chartData.length === 0) return null;
  return <LazyHistoryCharts data={chartData} />;
}

// ─── Wer gebucht hat (#4761) ─────────────────────────────────────────────────

function AttendanceActorSummary({
  checkedIn,
  checkedOut,
}: {
  readonly checkedIn: AttendanceActor | null;
  readonly checkedOut: AttendanceActor | null;
}) {
  const parts = [
    checkedIn ? `Angemeldet: ${attendanceActorLabel(checkedIn)}` : null,
    checkedOut ? `Abgemeldet: ${attendanceActorLabel(checkedOut)}` : null,
  ].filter((part): part is string => Boolean(part));
  if (parts.length === 0) return null;
  return (
    <span className="mt-0.5 block text-xs text-gray-600">
      {parts.join(" · ")}
    </span>
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
        className={`flex w-full items-center justify-between px-4 py-3 text-left transition-colors hover:bg-gray-50 ${isToday ? "bg-moto-blue-soft/60" : ""}`}
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
              <span className="text-moto-amber-strong ml-2 text-xs font-medium">
                {statusLabel}
              </span>
            )}
            {day.attendance && (
              <AttendanceActorSummary
                checkedIn={day.attendance.checkedInActor}
                checkedOut={day.attendance.checkedOutActor}
              />
            )}
            {day.attendance?.checkOutNote && (
              <span className="mt-0.5 block text-xs text-gray-600">
                Grund für das frühe Gehen: {day.attendance.checkOutNote}
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
            <span className="bg-moto-amber-soft text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
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
                  {/* Korrigieren nur mit Recht und nach Abschluss. Der fehlende
                      Instanzstatus synthetischer "Ohne Zuordnung"-Zeilen
                      schließt sie ebenfalls aus. */}
                  {canCorrect && slot.instanceStatus === "completed" && (
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
                  <span className="text-moto-amber-strong font-medium">
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
  const todayKey = useBerlinToday();
  // Korrigieren darf nur, wer den Plan verwaltet (#2898) — dieselbe
  // Berechtigung, die das Backend auf der Korrektur-Route verlangt.
  const { data: session } = useSession();
  const canCorrect = hasPermission(session, "schedules:manage");
  const [correcting, setCorrecting] = useState<CorrectableSlot | null>(null);

  return (
    <>
      {/* bodyClassName hebt die Kartenpolsterung wieder auf: die Tabelle bringt
          ihre eigene Zellpolsterung mit und läuft randlos bis zur Kartenkante. */}
      <SectionCard
        title="Anwesenheitsprotokoll"
        description={`${ROOM_HISTORY_DESCRIPTION} Letzte ${caps.attendanceDays} Tage · Raumdetails für ${caps.roomDetailDays} Tage`}
        leading={
          <ConceptIconTile concept={changeHistoryConcept} variant="section" />
        }
        bodyClassName="mt-4 -mx-5 -mb-5"
      >
        {days.length === 0 ? null : (
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
                          className={`cursor-pointer text-sm transition-colors hover:bg-gray-50 ${isToday ? "bg-moto-blue-soft/60" : ""}`}
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
                                <span className="bg-moto-amber-soft text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
                                  {statusLabel}
                                </span>
                              )}
                            </div>
                          </td>
                          <td className="px-6 py-3 text-gray-600 tabular-nums">
                            {day.attendance
                              ? formatTime(day.attendance.checkInTime)
                              : "–"}
                            {day.attendance?.checkedInActor && (
                              <span className="mt-0.5 block text-xs text-gray-500">
                                Angemeldet:{" "}
                                {attendanceActorLabel(
                                  day.attendance.checkedInActor,
                                )}
                              </span>
                            )}
                          </td>
                          <td className="px-6 py-3 text-gray-600 tabular-nums">
                            {day.attendance
                              ? day.attendance.checkOutTime
                                ? formatTime(day.attendance.checkOutTime)
                                : "Noch anwesend"
                              : "–"}
                            {day.attendance?.checkOutTime &&
                              day.attendance.checkedOutActor && (
                                <span className="mt-0.5 block text-xs text-gray-500">
                                  Abgemeldet:{" "}
                                  {attendanceActorLabel(
                                    day.attendance.checkedOutActor,
                                  )}
                                </span>
                              )}
                            {day.attendance?.checkOutNote && (
                              <span className="mt-0.5 block text-xs text-gray-500">
                                Grund für das frühe Gehen:{" "}
                                {day.attendance.checkOutNote}
                              </span>
                            )}
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
                                  klickbar. Korrigieren ist nur mit Recht und
                                  nach Abschluss des Termins sichtbar. */}
                                {slot.note && (
                                  <p className="mt-1 pl-4 text-gray-500 italic">
                                    Bemerkung: {slot.note}
                                  </p>
                                )}
                                {canCorrect &&
                                  slot.instanceStatus === "completed" && (
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
                                  <td className="text-moto-amber-strong py-2 pr-6 pl-12 font-medium">
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
                                  <td className="text-moto-amber-strong py-2 pr-6 pl-12 font-medium">
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
      </SectionCard>
      {correcting && (
        <AttendanceCorrectionModal
          isOpen
          onClose={() => setCorrecting(null)}
          studentId={studentId}
          slot={correcting}
          onCorrected={onCorrected}
        />
      )}
    </>
  );
}

// ─── Page ────────────────────────────────────────────────────────────────────

export default function StudentRoomHistoryPage() {
  return (
    <Suspense fallback={<RoomHistorySkeleton />}>
      <StudentRoomHistoryPageContent />
    </Suspense>
  );
}

function StudentRoomHistoryPageContent() {
  const today = useBerlinToday();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = resolveStudentReferrer(searchParams.get("from"));
  useSession();

  const [student, setStudent] = useState<Student | null>(null);
  const [history, setHistory] = useState<AttendanceHistory | null>(null);
  const [loading, setLoading] = useState(true);
  // A 404 is its own state, not an error to retry.
  const [notFound, setNotFound] = useState(false);
  const historyLoad = useApiLoadError();
  const showHistoryError = historyLoad.show;
  const clearHistoryError = historyLoad.clear;
  // Bumped by „Wiederholen“ to load the page again.
  const [attempt, setAttempt] = useState(0);
  const [exportOpen, setExportOpen] = useState(false);

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

  // Start at the top instead of inheriting the previous page's scroll position
  useScrollToTop(studentId);

  const fetchStudent = useCallback(async (): Promise<Student | null> => {
    try {
      const res = await transportFetch(`/api/students/${studentId}`);
      // The name only decorates the header; without it the page still shows
      // the protocol under its generic title.
      if (!res.ok) return null;
      const body = (await res.json()) as { data?: Student };
      return body.data ?? null;
    } catch (err) {
      // Same as above: the header falls back to the generic title.
      logger.error("student_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      return null;
    }
  }, [studentId]);

  const fetchHistory = useCallback(async (): Promise<void> => {
    try {
      const res = await transportFetch(
        `/api/students/${studentId}/attendance-history`,
      );
      if (res.status === 404) {
        setNotFound(true);
        setHistory(null);
        clearHistoryError();
        return;
      }
      if (!res.ok) {
        throw await apiErrorFromResponse(res, "attendance history failed");
      }
      const body = (await res.json()) as {
        data: BackendAttendanceHistoryResponse;
      };
      setHistory(mapAttendanceHistoryResponse(body.data));
      setNotFound(false);
      clearHistoryError();
    } catch (err) {
      logger.error("attendance_history_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      setHistory(null);
      await showHistoryError(err, {
        object: "das Anwesenheitsprotokoll",
        retry: () => setAttempt((current) => current + 1),
      });
    }
  }, [clearHistoryError, showHistoryError, studentId]);

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
  }, [fetchStudent, fetchHistory, attempt]);

  const displayName = student
    ? (student.name ?? `${student.first_name} ${student.second_name}`)
    : "";
  // Statuszeile: Klasse, Gruppe und die Zahl der protokollierten Tage, alles
  // aus den Daten, die die Seite ohnehin geladen hat. Ohne geladenes
  // Protokoll steht keine "0 Tage erfasst" neben dem Ladefehler (#2517).
  const dayCount = history ? history.days.length : null;
  const dayLine =
    dayCount === null
      ? null
      : `${dayCount} ${dayCount === 1 ? "Tag" : "Tage"} erfasst`;
  const studentMeta = student
    ? [student.school_class, student.group_name, dayLine]
        .filter(Boolean)
        .join(" · ")
    : "";
  // Eine ausgeschaltete Funktion ist ein Zustand, kein Fehler. Erkannt wird
  // sie am Schalter im Datensatz des Kindes, nicht am Text der 403-Antwort.
  const featureDisabled = student?.attendance_log_enabled === false;
  const pageError = notFound
    ? "Kind nicht gefunden."
    : featureDisabled
      ? null
      : historyLoad.error;
  const back = studentHistoryBack(studentId, referrer, pageError !== null);

  return (
    <>
      {/* Der Entitätskopf ist die Kopfkarte der Seite. */}
      <TenantPage
        back
        {...back}
        leading={
          <ConceptIconTile concept={changeHistoryConcept} variant="page" />
        }
        title={displayName || "Anwesenheitsprotokoll"}
        stats={studentMeta || dayLine}
        statsLoading={loading}
        loading={loading}
        error={pageError}
        // Exportieren steht im Kebab der Kopfkarte, wie auf jeder anderen
        // Werkzeugfläche; der Zeitraum wird im Export-Dialog gewählt.
        actions={
          history && history.days.length > 0 ? (
            <OverflowMenu
              items={[
                {
                  label: "Exportieren",
                  icon: <Download className="h-4 w-4" aria-hidden />,
                  onClick: () => setExportOpen(true),
                },
              ]}
              ariaLabel="Weitere Aktionen"
            />
          ) : undefined
        }
        // Eine ausgeschaltete Funktion ist ein Zustand, kein Fehler; ebenso ein
        // Zeitraum ohne Eintrag. Beide nennen den nächsten Schritt.
        empty={
          featureDisabled
            ? {
                title: "Anwesenheitsprotokoll ist ausgeschaltet",
                description: FEATURE_DISABLED_DESCRIPTION,
              }
            : history && history.days.length === 0
              ? {
                  title:
                    "Keine Anwesenheitsdaten für den ausgewählten Zeitraum verfügbar",
                  description: `Letzte ${history.caps.attendanceDays} Tage · Raumdetails für ${history.caps.roomDetailDays} Tage. Sobald das Kind an- oder abgemeldet wird, erscheint der Tag hier.`,
                }
              : null
        }
      >
        {history && history.days.length > 0 && (
          <>
            <HistoryCharts days={history.days} />

            <HistoryTable
              days={history.days}
              caps={history.caps}
              studentId={studentId}
              onCorrected={() => void fetchHistory()}
            />
          </>
        )}
      </TenantPage>
      {history && exportOpen ? (
        <AttendanceExportDialog
          isOpen
          onClose={() => setExportOpen(false)}
          studentId={studentId}
          studentName={displayName}
          today={today}
          visibleDays={history.caps.attendanceDays}
        />
      ) : null}
    </>
  );
}
