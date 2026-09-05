"use client";

import { useCallback, useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { getCachedSession } from "~/lib/session-cache";
import {
  formatAttendanceSlotStatus,
  type AttendanceSlotStatus,
} from "~/lib/attendance-history-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentHistorieTab" });

interface StudentHistorieTabProps {
  studentId: string;
}

interface AttendanceDayRecord {
  check_in_time: string;
  check_out_time?: string | null;
  duration_minutes?: number | null;
}

interface AttendanceVisitEntry {
  room_id?: number | null;
  room_name?: string;
  entry_time: string;
  exit_time?: string | null;
  duration_minutes?: number | null;
}

interface AttendanceHistoryDay {
  date: string;
  attendance: AttendanceDayRecord | null;
  room_detail_available: boolean;
  visits: AttendanceVisitEntry[];
  slots?: Array<{
    instance_id: string;
    title: string;
    start_time: string;
    end_time: string;
    status: AttendanceSlotStatus;
    substatus?: string | null;
    is_unplanned: boolean;
  }>;
}

interface AttendanceHistoryResponse {
  student_id: string;
  days: AttendanceHistoryDay[];
  range: { start: string; end: string };
  clamped: boolean;
  caps: { attendance_days: number; room_detail_days: number };
}

type LoadState =
  | { status: "loading" }
  | { status: "disabled" }
  | { status: "forbidden" }
  | { status: "error"; message: string }
  | { status: "ready"; data: AttendanceHistoryResponse };

function formatDateLabel(isoDate: string): string {
  const date = new Date(`${isoDate}T00:00:00`);
  if (Number.isNaN(date.getTime())) return isoDate;
  return date.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    weekday: "long",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatTime(isoString: string): string {
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return "–";
  return date.toLocaleTimeString("de-DE", {
    timeZone: "Europe/Berlin",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatDuration(minutes: number | null | undefined): string {
  if (typeof minutes !== "number" || minutes <= 0) return "–";
  const hours = Math.floor(minutes / 60);
  const remaining = minutes % 60;
  if (hours === 0) return `${remaining} min`;
  return `${hours}h ${remaining.toString().padStart(2, "0")}m`;
}

export function StudentHistorieTab({ studentId }: StudentHistorieTabProps) {
  const [state, setState] = useState<LoadState>({ status: "loading" });

  const loadData = useCallback(async () => {
    setState({ status: "loading" });
    try {
      const session = await getCachedSession();
      const headers: Record<string, string> = {};
      if (session?.user?.token) {
        headers.Authorization = `Bearer ${session.user.token}`;
      }
      const response = await fetch(
        `/api/students/${studentId}/attendance-history`,
        { credentials: "include", headers },
      );
      if (response.status === 403) {
        const text = await response.text().catch(() => "");
        if (text.includes("feature_disabled")) {
          setState({ status: "disabled" });
          return;
        }
        setState({ status: "forbidden" });
        return;
      }
      if (!response.ok) {
        const text = await response.text().catch(() => "");
        throw new Error(
          text
            ? `Request failed (${response.status}): ${text}`
            : `Request failed (${response.status})`,
        );
      }
      const payload = (await response.json()) as
        { data: AttendanceHistoryResponse } | AttendanceHistoryResponse;
      const data =
        "data" in payload && payload.data
          ? payload.data
          : (payload as AttendanceHistoryResponse);
      setState({ status: "ready", data });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to load attendance history", {
        studentId,
        error: message,
      });
      setState({ status: "error", message });
    }
  }, [studentId]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  if (state.status === "loading") {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-gray-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden />
        Lade Anwesenheits-Historie...
      </div>
    );
  }

  if (state.status === "disabled") {
    return (
      <div className="rounded-xl border border-gray-200 bg-gray-50 p-6 text-center">
        <MotoConceptIcon
          concept="changeHistory"
          size={34}
          className="mx-auto mb-2"
        />
        <h3 className="text-sm font-semibold text-gray-900">
          Historie ist deaktiviert
        </h3>
        <p className="mt-1 text-xs text-gray-600">
          Die Anwesenheits-Historie ist für diese Einrichtung nicht
          freigeschaltet. Admin kann sie in den Einstellungen unter Datenschutz
          aktivieren.
        </p>
      </div>
    );
  }

  if (state.status === "forbidden") {
    return (
      <div className="border-moto-amber/30 bg-moto-amber/10 text-moto-amber-strong rounded-xl border p-6 text-center text-sm">
        Du hast keine Berechtigung, diese Historie zu sehen.
      </div>
    );
  }

  if (state.status === "error") {
    return (
      <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-4 text-sm">
        Historie konnte nicht geladen werden. {state.message}
      </div>
    );
  }

  const { data } = state;
  const daysWithData = data.days.filter(
    (day) => day.attendance !== null || (day.slots?.length ?? 0) > 0,
  );

  return (
    <SectionCard
      title="Anwesenheits-Historie"
      titleClassName="text-sm"
      headingLevel={3}
      description={`Letzte ${data.caps.attendance_days} Tage. Raum-Details für ${data.caps.room_detail_days} Tage sichtbar.`}
      bodyClassName="mt-4"
    >
      {daysWithData.length === 0 ? (
        <EmptyState
          variant="compact"
          title="Keine Einträge im sichtbaren Zeitraum"
        />
      ) : (
        <ul className="space-y-2" role="list">
          {daysWithData.map((day) => (
            <li
              key={day.date}
              className="moto-content-surface rounded-lg border p-4"
            >
              <div className="flex items-center justify-between">
                <div className="text-sm font-semibold text-gray-900">
                  {formatDateLabel(day.date)}
                </div>
                <div className="text-xs text-gray-500">
                  {day.attendance?.check_in_time
                    ? formatTime(day.attendance.check_in_time)
                    : "–"}{" "}
                  –{" "}
                  {day.attendance?.check_out_time
                    ? formatTime(day.attendance.check_out_time)
                    : "offen"}
                  {typeof day.attendance?.duration_minutes === "number"
                    ? ` · ${formatDuration(day.attendance.duration_minutes)}`
                    : null}
                </div>
              </div>
              {(day.slots?.length ?? 0) > 0 ? (
                <ul className="mt-2 space-y-1 border-t border-gray-100 pt-2">
                  {day.slots?.map((slot) => (
                    <li
                      key={slot.instance_id}
                      className="flex items-center justify-between text-xs text-gray-600"
                    >
                      <span>
                        {slot.title}
                        {slot.is_unplanned ? " · ungeplant" : ""}
                      </span>
                      <span>
                        {slot.start_time}–{slot.end_time} ·{" "}
                        {formatAttendanceSlotStatus(
                          slot.status,
                          slot.substatus,
                        ).toLocaleLowerCase("de-DE")}
                      </span>
                    </li>
                  ))}
                </ul>
              ) : null}
              {day.room_detail_available && day.visits.length > 0 ? (
                <ul className="mt-2 space-y-1 border-t border-gray-100 pt-2">
                  {day.visits.map((visit, index) => (
                    <li
                      key={`${day.date}-${index}`}
                      className="flex items-center justify-between text-xs text-gray-600"
                    >
                      <span>{visit.room_name || "Raum unbekannt"}</span>
                      <span>
                        {formatTime(visit.entry_time)} –{" "}
                        {visit.exit_time
                          ? formatTime(visit.exit_time)
                          : "offen"}
                      </span>
                    </li>
                  ))}
                </ul>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
