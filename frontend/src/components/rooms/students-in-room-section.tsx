// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view rendered on /rooms/{id} (#1323).
//
// Reuse-only: composes InfoCard + StudentCard + Button + Alert + Loading.
// No bespoke UI primitives live here — every visual element is the same
// component the rest of the app uses, so the section stays consistent
// when those primitives evolve.
//
// Data path:
//   1. Initial paint via useSWRAuth("rooms-students-present-{roomId}").
//   2. SSE-driven refresh: useGlobalSSE (mounted in tenant-auth-wrapper)
//      invalidates this SWR key on student_checkin / student_checkout /
//      activity_start / activity_end / dashboard_counts_changed events.
//
// GDPR: redaction is server-side. Rows with hasFullAccess=false arrive with
// names/group/entry_time set to null. We still render them via StudentCard
// (labelled "Anonymisiert", click is a no-op, no group/time row) so the
// total count stays consistent with what a kiosk colleague sees and SSE
// diffs by studentId still work.

"use client";

import { useSession } from "next-auth/react";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSWRAuth } from "~/lib/swr";
import { InfoCard } from "~/components/ui/info-card";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import {
  StudentCard,
  StudentInfoRow,
  GroupIcon,
  PickupTimeIcon,
} from "~/components/students/student-card";
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

function formatEntryTime(iso: string | null): string | null {
  if (!iso) return null;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleTimeString("de-DE", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function StudentsInRoomSection({
  roomId,
  roomName,
}: StudentsInRoomSectionProps) {
  const router = useTenantRouter();
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

  const openInSearch = () => {
    const qs = new URLSearchParams({
      room_id: roomId,
      room_name: roomName,
    }).toString();
    router.push(`/students/search?${qs}`);
  };

  return (
    <InfoCard
      title="Kinder im Raum"
      icon={
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
          />
        </svg>
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-gray-600">
          <span className="font-medium text-gray-900">{totalCount}</span>{" "}
          {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
          {visibleCount < totalCount && (
            <span className="ml-1 text-gray-500">
              · {visibleCount} von {totalCount} sichtbar · Du siehst nur Kinder
              aus deinen Gruppen.
            </span>
          )}
        </p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={openInSearch}
        >
          In Kindersuche öffnen
        </Button>
      </div>

      <StudentsInRoomBody
        roomId={roomId}
        loading={isLoading}
        hasError={!!error}
        students={students}
      />
    </InfoCard>
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
      <Alert
        type="error"
        message="Die Liste der Kinder konnte nicht geladen werden."
      />
    );
  }

  if (loading && students.length === 0) {
    return <Loading message="Kinderliste wird geladen..." fullPage={false} />;
  }

  if (students.length === 0) {
    // Match the "Keine Belegungshistorie verfügbar." pattern already used by
    // the other InfoCard on this page — same typography, same spacing.
    return (
      <div className="py-8 text-center text-gray-500">
        Aktuell keine Kinder im Raum.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {students.map((s) => (
        <StudentInRoomCard key={s.studentId} student={s} roomId={roomId} />
      ))}
    </div>
  );
}

interface StudentInRoomCardProps {
  readonly student: StudentInRoom;
  readonly roomId: string;
}

function StudentInRoomCard({ student, roomId }: StudentInRoomCardProps) {
  const router = useTenantRouter();
  const isRedacted = !student.hasFullAccess;
  const entryTime = formatEntryTime(student.entryTime);

  // For redacted rows we keep the row visible (count parity with the kiosk
  // view) but show no PII and no navigation: clicking is a no-op and no
  // group / entry time is rendered.
  return (
    <StudentCard
      studentId={student.studentId}
      firstName={isRedacted ? "Anonymisiert" : (student.firstName ?? "")}
      lastName={isRedacted ? "" : (student.lastName ?? "")}
      onClick={() => {
        if (isRedacted) return;
        router.push(`/students/${student.studentId}?from=/rooms/${roomId}`);
      }}
      locationBadge={null}
      extraContent={
        isRedacted ? null : (
          <>
            {student.groupName && (
              <StudentInfoRow icon={<GroupIcon />}>
                Gruppe: {student.groupName}
              </StudentInfoRow>
            )}
            {entryTime && (
              <StudentInfoRow icon={<PickupTimeIcon />}>
                seit {entryTime} Uhr
              </StudentInfoRow>
            )}
          </>
        )
      }
    />
  );
}
