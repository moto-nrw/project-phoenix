"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { Alert } from "~/components/ui/alert";
import { useSession } from "next-auth/react";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";
import {
  type AttendanceHistory,
  type BackendAttendanceHistoryResponse,
  formatDate,
  formatDuration,
  formatTime,
  mapAttendanceHistoryResponse,
} from "~/lib/attendance-history-helpers";

const logger = createLogger({ component: "StudentRoomHistoryPage" });

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
  | "feature_disabled"
  | "not_group_supervisor"
  | "not_found"
  | "generic";

const ERROR_MESSAGES: Record<ErrorCode, string> = {
  feature_disabled:
    "Diese Funktion ist für Ihre Schule deaktiviert. Bitte wenden Sie sich an Ihre Administration.",
  not_group_supervisor:
    "Sie können nur das Anwesenheitsprotokoll von Schülern Ihrer betreuten Gruppen einsehen.",
  not_found: "Schüler nicht gefunden.",
  generic: "Fehler beim Laden des Anwesenheitsprotokolls.",
};

export default function StudentRoomHistoryPage() {
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

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

  const fetchStudent = useCallback(async (): Promise<Student | null> => {
    try {
      const res = await fetch(`/api/students/${studentId}`);
      if (!res.ok) {
        return null;
      }
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
    return <Loading fullPage={false} />;
  }

  if (errorCode !== null && errorCode !== "feature_disabled") {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center">
        <Alert type="error" message={ERROR_MESSAGES[errorCode]} />
        <button
          onClick={() => router.push(referrer)}
          className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
        >
          Zurück
        </button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl">
      {/* Back Button */}
      <div className="mb-6">
        <button
          onClick={() => router.push(`/students/${studentId}?from=${referrer}`)}
          className="flex items-center text-gray-600 transition-colors hover:text-blue-600"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="mr-1 h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M10 19l-7-7m0 0l7-7m-7 7h18"
            />
          </svg>
          Zurück zum Schülerprofil
        </button>
      </div>

      {/* Student Profile Header */}
      {student && (
        <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white shadow-md">
          <div className="flex items-center">
            <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
              {student.first_name[0]}
              {student.second_name[0]}
            </div>
            <div>
              <h1 className="text-3xl font-bold">
                {student.name ?? `${student.first_name} ${student.second_name}`}
              </h1>
              <div className="mt-1 flex items-center">
                <span className="opacity-90">
                  Klasse {student.school_class}
                </span>
                {student.group_name && (
                  <>
                    <span className="mx-2">•</span>
                    <span className="opacity-90">
                      Gruppe: {student.group_name}
                    </span>
                  </>
                )}
              </div>
              <div className="mt-4">
                <h2 className="text-2xl font-semibold text-white">
                  Anwesenheitsprotokoll & Raumverlauf
                </h2>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Feature disabled banner */}
      {errorCode === "feature_disabled" && (
        <div className="mb-6 rounded-lg border border-amber-200 bg-amber-50 p-4 text-amber-900">
          {ERROR_MESSAGES.feature_disabled}
        </div>
      )}

      {history && (
        <>
          {/* Info banner: caps + clamped notice */}
          <div className="mb-6 space-y-2">
            <div className="rounded-lg border border-blue-100 bg-blue-50 p-3 text-sm text-blue-900">
              Anzeige beschränkt auf die letzten {history.caps.attendanceDays}{" "}
              Tage Anwesenheit. Raumdetails sind für die letzten{" "}
              {history.caps.roomDetailDays} Tage sichtbar.
            </div>
            {history.clamped && (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                Der angeforderte Zeitraum überschreitet das konfigurierte
                Maximum und wurde automatisch gekürzt.
              </div>
            )}
          </div>

          {/* Days timeline */}
          <div className="space-y-6">
            {history.days.length === 0 ? (
              <div className="rounded-lg bg-white p-8 text-center shadow-sm">
                <p className="text-gray-500">
                  Keine Anwesenheitsdaten für den ausgewählten Zeitraum
                  verfügbar.
                </p>
              </div>
            ) : (
              history.days.map((day) => (
                <div
                  key={day.date}
                  className="overflow-hidden rounded-lg bg-white shadow-sm"
                >
                  <div className="border-b border-blue-100 bg-blue-50 px-6 py-3">
                    <h3 className="font-medium text-blue-800">
                      {formatDate(day.date)}
                    </h3>
                  </div>

                  {/* Attendance band */}
                  {day.attendance && (
                    <div className="border-b border-gray-100 px-6 py-4">
                      <div className="flex flex-wrap items-center gap-4 text-sm">
                        <div>
                          <span className="text-gray-500">Ankunft: </span>
                          <span className="font-medium text-gray-900">
                            {formatTime(day.attendance.checkInTime)}
                          </span>
                        </div>
                        <div>
                          <span className="text-gray-500">Abmeldung: </span>
                          <span className="font-medium text-gray-900">
                            {day.attendance.checkOutTime
                              ? formatTime(day.attendance.checkOutTime)
                              : "Noch anwesend"}
                          </span>
                        </div>
                        <div>
                          <span className="text-gray-500">Dauer: </span>
                          <span className="font-medium text-gray-900">
                            {formatDuration(day.attendance.durationMinutes)}
                          </span>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Room detail timeline (or unavailable note) */}
                  <div className="px-6 py-2">
                    {day.roomDetailAvailable ? (
                      day.visits.length === 0 ? (
                        <p className="py-2 text-sm text-gray-500">
                          Keine Raumwechsel an diesem Tag.
                        </p>
                      ) : (
                        <ul className="relative">
                          {day.visits.map((v, index) => (
                            <li
                              key={`${day.date}-${index}-${v.entryTime.toISOString()}`}
                              className="relative border-l-2 border-gray-200 py-4 pl-8"
                            >
                              <div className="absolute -left-[5px] mt-1.5 h-3 w-3 rounded-full bg-blue-500"></div>
                              <div className="flex flex-col space-y-1">
                                <span className="text-sm text-gray-500">
                                  {formatTime(v.entryTime)}
                                  {v.exitTime && (
                                    <> – {formatTime(v.exitTime)}</>
                                  )}
                                </span>
                                <span className="font-medium text-gray-900">
                                  {v.roomName || "Unbekannter Raum"}
                                </span>
                                {v.durationMinutes != null && (
                                  <span className="text-sm text-gray-600">
                                    Dauer: {formatDuration(v.durationMinutes)}
                                  </span>
                                )}
                              </div>
                            </li>
                          ))}
                        </ul>
                      )
                    ) : (
                      <p className="py-2 text-sm text-gray-500 italic">
                        Raumdetails für diesen Tag nicht mehr verfügbar
                        (Aufbewahrungsfrist überschritten).
                      </p>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </>
      )}
    </div>
  );
}
