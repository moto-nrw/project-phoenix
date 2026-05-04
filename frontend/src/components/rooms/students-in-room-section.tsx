// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view rendered on /rooms/{id} below the room info
// card and above the history block (see plan in docs/room-students-present-plan.md).
//
// Data path:
//   1. Initial paint via useSWRAuth("rooms-students-present-{roomId}").
//   2. SSE-driven refresh: useGlobalSSE (mounted in tenant-auth-wrapper)
//      invalidates this SWR key on student_checkin / student_checkout /
//      activity_start / activity_end / dashboard_counts_changed events.
//      We do NOT call useSSE here — that would open a second EventSource.
//
// GDPR: redaction is server-side. Rows with hasFullAccess=false arrive with
// names/group/entry_time set to null; we render a generic placeholder card
// for them instead of dropping the row, so totalCount stays consistent
// with what a kiosk colleague sees and SSE diffs by studentId still work.

"use client";

import Link from "next/link";
import { useSession } from "next-auth/react";
import { Lock } from "lucide-react";
import { useSWRAuth } from "~/lib/swr";
import {
  mapStudentsInRoomResponse,
  type BackendStudentsInRoomResponse,
  type StudentInRoom,
  type StudentsInRoomResponse,
} from "~/lib/students-in-room-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsInRoomSection" });

interface StudentsInRoomSectionProps {
  readonly roomId: string;
  readonly roomName: string;
}

interface ApiEnvelope<T> {
  data?: T;
}

async function fetchStudentsInRoom(
  roomId: string,
  token: string | undefined,
): Promise<StudentsInRoomResponse> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (token) headers.Authorization = `Bearer ${token}`;

  const response = await fetch(`/api/rooms/${roomId}/students-present`, {
    credentials: "include",
    headers,
  });

  if (!response.ok) {
    throw new Error(`students_present_http_${response.status}`);
  }

  const json =
    (await response.json()) as ApiEnvelope<BackendStudentsInRoomResponse>;
  if (!json.data) {
    throw new Error("students_present_missing_data");
  }
  return mapStudentsInRoomResponse(json.data);
}

export function StudentsInRoomSection({
  roomId,
  roomName,
}: StudentsInRoomSectionProps) {
  const { data: session } = useSession();
  const token = session?.user?.token;

  const { data, error, isLoading } = useSWRAuth<StudentsInRoomResponse>(
    `rooms-students-present-${roomId}`,
    () => fetchStudentsInRoom(roomId, token),
  );

  if (error) {
    logger.warn("students_present_load_failed", {
      room_id: roomId,
      error: error instanceof Error ? error.message : String(error),
    });
  }

  const totalCount = data?.totalCount ?? 0;
  const visibleCount = data?.visibleCount ?? 0;
  const students = data?.students ?? [];

  const kindersucheHref = `/students/search?room_id=${encodeURIComponent(roomId)}&room_name=${encodeURIComponent(roomName)}`;

  return (
    <section
      aria-labelledby="kinder-im-raum-heading"
      className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6"
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2
            id="kinder-im-raum-heading"
            className="text-lg font-semibold text-gray-900"
          >
            Kinder im Raum
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            <span className="font-medium text-gray-900">{totalCount}</span>{" "}
            {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
            {visibleCount < totalCount ? (
              <span className="ml-1 text-gray-500">
                · {visibleCount} von {totalCount} sichtbar · Du siehst nur
                Kinder aus deinen Gruppen.
              </span>
            ) : null}
          </p>
        </div>

        <Link
          href={kindersucheHref}
          className="inline-flex items-center rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus:ring-2 focus:ring-offset-2 focus:outline-none"
        >
          In Kindersuche öffnen
        </Link>
      </div>

      <StudentsInRoomBody
        roomId={roomId}
        loading={isLoading}
        hasError={!!error}
        students={students}
      />
    </section>
  );
}

interface StudentsInRoomBodyProps {
  readonly roomId: string;
  readonly loading: boolean;
  readonly hasError: boolean;
  readonly students: readonly StudentInRoom[];
}

function StudentsInRoomBody({
  roomId,
  loading,
  hasError,
  students,
}: StudentsInRoomBodyProps) {
  if (hasError) {
    return (
      <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">
        Die Liste der Kinder konnte nicht geladen werden.
      </div>
    );
  }

  if (loading && students.length === 0) {
    return (
      <div
        aria-busy="true"
        aria-label="Kinderliste wird geladen"
        className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3"
      >
        {Array.from({ length: 3 }).map((_, i) => (
          <div
            key={i}
            className="h-20 animate-pulse rounded-lg border border-gray-100 bg-gray-50"
          />
        ))}
      </div>
    );
  }

  if (students.length === 0) {
    return (
      <div className="rounded-md border border-gray-100 bg-gray-50 p-4 text-sm text-gray-600">
        Aktuell keine Kinder im Raum.
      </div>
    );
  }

  return (
    <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {students.map((s) => (
        <li key={s.studentId}>
          <StudentInRoomCard student={s} roomId={roomId} />
        </li>
      ))}
    </ul>
  );
}

interface StudentInRoomCardProps {
  readonly student: StudentInRoom;
  readonly roomId: string;
}

function StudentInRoomCard({ student, roomId }: StudentInRoomCardProps) {
  if (!student.hasFullAccess) {
    // Redacted row: the count must still match the kiosk view, so we render
    // a placeholder. No name, no link, no group.
    return (
      <div
        aria-label="Kind ohne Sichtbarkeit"
        className="flex items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500"
      >
        <Lock className="h-4 w-4" aria-hidden="true" />
        <span>Kind ohne Sichtbarkeit</span>
      </div>
    );
  }

  const fullName =
    `${student.firstName ?? ""} ${student.lastName ?? ""}`.trim();
  const entryLabel = student.entryTime
    ? new Date(student.entryTime).toLocaleTimeString("de-DE", {
        hour: "2-digit",
        minute: "2-digit",
      })
    : null;

  return (
    <Link
      href={`/students/${student.studentId}?from=/rooms/${roomId}`}
      className="block rounded-lg border border-gray-200 bg-white p-4 text-left transition-colors hover:border-gray-300 hover:bg-gray-50 focus:ring-2 focus:ring-offset-2 focus:outline-none"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate font-medium text-gray-900">
            {fullName || "—"}
          </div>
          {student.groupName ? (
            <div className="mt-0.5 truncate text-xs text-gray-500">
              {student.groupName}
            </div>
          ) : null}
        </div>
        {entryLabel ? (
          <span className="text-xs whitespace-nowrap text-gray-500">
            seit {entryLabel}
          </span>
        ) : null}
      </div>
    </Link>
  );
}
